package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/client/partition"
	"github.com/metal-stack/metal-go/api/client/project"
	"github.com/metal-stack/metal-go/api/client/switch_operations"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	metaltag "github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
)

func update(updateTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(2) // to be graceful with the metal-api

	g.Go(func() error { return networkMetrics(ctx) })
	g.Go(func() error { return partitionMetrics(ctx) })
	g.Go(func() error { return imageMetrics(ctx) })
	g.Go(func() error { return projectMetrics(ctx) })
	g.Go(func() error { return switchMetrics(ctx) })
	g.Go(func() error { return machineMetrics(ctx) })

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error during metrics update: %w", err)
	}

	return nil
}

var (
	networkInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_network_info",
		Help: "Provide information about the network",
	}, []string{"networkId", "name", "projectId", "description", "partition", "vrf", "prefixes", "destPrefixes", "parentNetworkID", "isPrivateSuper", "useNat", "isUnderlay"})
	usedIPs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_network_ip_used",
		Help: "The total number of used IPs of the network",
	}, []string{"networkId"})
	availableIps = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_network_ip_available",
		Help: "The total number of available IPs of the network",
	}, []string{"networkId"})
	usedPrefixes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_network_prefix_used",
		Help: "The total number of used prefixes of the network",
	}, []string{"networkId"})
	availablePrefixes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_network_prefix_available",
		Help: "The total number of available prefixes of the network",
	}, []string{"networkId"})
)

func networkMetrics(ctx context.Context) error {
	resp, err := client.Network().ListNetworks(network.NewListNetworksParams().WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving networks: %w", err)
	}

	for _, nw := range resp.Payload {
		if nw.Privatesuper == nil || nw.Nat == nil || nw.Underlay == nil || nw.ID == nil {
			continue
		}

		var (
			nwID         = pointer.SafeDeref(nw.ID)
			privateSuper = fmt.Sprintf("%t", *nw.Privatesuper)
			nat          = fmt.Sprintf("%t", *nw.Nat)
			underlay     = fmt.Sprintf("%t", *nw.Underlay)
			prefixes     = strings.Join(nw.Prefixes, ",")
			destPrefixes = strings.Join(nw.Destinationprefixes, ",")
			vrf          = fmt.Sprintf("%d", nw.Vrf)
		)

		networkInfo.WithLabelValues(
			nwID, nw.Name, nw.Projectid, nw.Description, nw.Partitionid, vrf, prefixes, destPrefixes, nw.Parentnetworkid, privateSuper, nat, underlay,
		).Set(1.0)

		if nw.Usage == nil {
			continue
		}

		usedIPs.WithLabelValues(nwID).Set(float64(pointer.SafeDeref(nw.Usage.UsedIps)))
		availableIps.WithLabelValues(nwID).Set(float64(pointer.SafeDeref(nw.Usage.AvailableIps)))
		usedPrefixes.WithLabelValues(nwID).Set(float64(pointer.SafeDeref(nw.Usage.UsedPrefixes)))
		availablePrefixes.WithLabelValues(nwID).Set(float64(pointer.SafeDeref(nw.Usage.AvailablePrefixes)))
	}

	return nil
}

var (
	capacityTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_total",
		Help: "The total number of machines in the partition",
	}, []string{"partition", "size"})
	capacityWaiting = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_waiting",
		Help: "The total number of waiting machines in the partition",
	}, []string{"partition", "size"})
	capacityFree = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_free",
		Help: "(DEPRECATED) The total number of allocatable machines in the partition, use metal_partition_capacity_allocatable",
	}, []string{"partition", "size"})
	capacityAllocatable = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_allocatable",
		Help: "The total number of waiting allocatable machines in the partition",
	}, []string{"partition", "size"})
	capacityAllocated = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_allocated",
		Help: "The capacity of allocated machines in the partition",
	}, []string{"partition", "size"})
	capacityReservationsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_reservations_total",
		Help: "The sum of capacity reservations in the partition",
	}, []string{"partition", "size"})
	capacityReservationsUsed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_reservations_used",
		Help: "The sum of used capacity reservations in the partition",
	}, []string{"partition", "size"})
	capacityPhonedHome = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_phoned_home",
		Help: "The total number of faulty machines in the partition",
	}, []string{"partition", "size"})
	capacityFaulty = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_faulty",
		Help: "The capacity of faulty machines in the partition",
	}, []string{"partition", "size"})
	capacityUnavailable = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_unavailable",
		Help: "The total number of unavailable machines in the partition",
	}, []string{"partition", "size"})
	capacityOther = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_partition_capacity_other",
		Help: "The total number of machines in an other state in the partition",
	}, []string{"partition", "size"})
)

func partitionMetrics(ctx context.Context) error {
	resp, err := client.Partition().PartitionCapacity(partition.NewPartitionCapacityParams().WithBody(&models.V1PartitionCapacityRequest{}).WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving partitions: %w", err)
	}

	for _, p := range resp.Payload {
		if p.ID == nil {
			continue
		}

		for _, s := range p.Servers {
			if s.Size == nil {
				continue
			}

			var (
				pID  = pointer.SafeDeref(p.ID)
				size = pointer.SafeDeref(s.Size)
			)

			capacityTotal.WithLabelValues(pID, size).Set(float64(s.Total))
			capacityAllocated.WithLabelValues(pID, size).Set(float64(s.Allocated))
			capacityWaiting.WithLabelValues(pID, size).Set(float64(s.Waiting))
			capacityFree.WithLabelValues(pID, size).Set(float64(s.Allocatable))
			capacityAllocatable.WithLabelValues(pID, size).Set(float64(s.Allocatable))
			capacityFaulty.WithLabelValues(pID, size).Set(float64(s.Faulty))
			capacityReservationsTotal.WithLabelValues(pID, size).Set(float64(s.Reservations))
			capacityReservationsUsed.WithLabelValues(pID, size).Set(float64(s.Usedreservations))
			capacityPhonedHome.WithLabelValues(pID, size).Set(float64(s.PhonedHome))
			capacityUnavailable.WithLabelValues(pID, size).Set(float64(s.Unavailable))
			capacityOther.WithLabelValues(pID, size).Set(float64(s.Other))
		}
	}

	return nil
}

var (
	usedImage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_image_used_total",
		Help: "The total number of machines using a image",
	}, []string{"imageID", "name", "classification", "created", "expirationDate", "features"})
)

func imageMetrics(ctx context.Context) error {
	resp, err := client.Image().ListImages(image.NewListImagesParams().WithShowUsage(pointer.Pointer(true)).WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving images: %w", err)
	}

	for _, i := range resp.Payload {
		if i.ID == nil {
			continue
		}

		var (
			id             = pointer.SafeDeref(i.ID)
			usage          = len(i.Usedby)
			created        = fmt.Sprintf("%d", time.Time(i.Created).Unix())
			expirationDate = fmt.Sprintf("%d", time.Time(pointer.SafeDeref(i.ExpirationDate)).Unix())
			features       = strings.Join(i.Features, ",")
		)

		usedImage.WithLabelValues(id, i.Name, i.Classification, created, expirationDate, features).Set(float64(usage))
	}

	return nil
}

var (
	projectInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_project_info",
		Help: "Provide information about metal projects",
	}, []string{"projectId", "name", "tenantId"})
)

func projectMetrics(ctx context.Context) error {
	resp, err := client.Project().ListProjects(project.NewListProjectsParams().WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving images: %w", err)
	}

	for _, p := range resp.Payload {
		projectInfo.WithLabelValues(p.Meta.ID, p.Name, p.TenantID).Set(1.0)
	}

	return nil
}

var (
	switchInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_switch_info",
		Help: "Provide information about the switch",
	}, []string{"switchname", "partition", "rackid", "metalCoreVersion", "osVendor", "osVersion", "managementIP"})
	switchInterfaceInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_switch_interface_info",
		Help: "Provide information about the switch interfaces",
	}, []string{"switchname", "device", "machineid", "partition"})
	switchMetalCoreUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_switch_metal_core_up",
		Help: "1 when the metal-core is up, otherwise 0",
	}, []string{"switchname", "partition", "rackid"})
	switchSyncFailed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_switch_sync_failed",
		Help: "1 when the switch sync is failing, otherwise 0",
	}, []string{"switchname", "partition", "rackid"})
	switchSyncDurationsMs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_switch_sync_durations_ms",
		Help: "The duration of the syncs in milliseconds",
	}, []string{"switchname", "partition", "rackid"})
)

func switchMetrics(ctx context.Context) error {
	resp, err := client.SwitchOperations().ListSwitches(switch_operations.NewListSwitchesParams().WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving switches: %w", err)
	}

	for _, s := range resp.Payload {
		var (
			lastSync      = time.Time(pointer.SafeDeref(pointer.SafeDeref(s.LastSync).Time))
			lastSyncError = time.Time(pointer.SafeDeref(pointer.SafeDeref(s.LastSyncError).Time))

			syncFailed              = 0.0
			lastSyncDurationMs      = float64(time.Duration(pointer.SafeDeref(pointer.SafeDeref(s.LastSync).Duration)).Milliseconds())
			lastSyncErrorDurationMs = float64(time.Duration(pointer.SafeDeref(pointer.SafeDeref(s.LastSyncError).Duration)).Milliseconds())

			partitionID = pointer.SafeDeref(pointer.SafeDeref(s.Partition).ID)
			rackID      = pointer.SafeDeref(s.RackID)
			osVendor    = pointer.SafeDeref(s.Os).Vendor
			osVersion   = pointer.SafeDeref(s.Os).Version
			// metal core version is very long: v0.9.1 (1d5e42ea), tags/v0.9.1-0-g1d5e42e, go1.20.5
			metalCoreVersion = strings.Split(pointer.SafeDeref(s.Os).MetalCoreVersion, ",")[0]
			metalCoreUp      = 1.0
			managementIP     = s.ManagementIP
		)

		if lastSyncError.After(lastSync) {
			syncFailed = 1.0
			lastSyncDurationMs = lastSyncErrorDurationMs
			lastSync = lastSyncError
		}

		if time.Since(lastSync) > 1*time.Minute {
			metalCoreUp = 0.0
		}

		switchMetalCoreUp.WithLabelValues(s.Name, partitionID, rackID).Set(metalCoreUp)
		switchSyncFailed.WithLabelValues(s.Name, partitionID, rackID).Set(syncFailed)
		switchInfo.WithLabelValues(s.Name, partitionID, rackID, metalCoreVersion, osVendor, osVersion, managementIP).Set(1.0)
		switchSyncDurationsMs.WithLabelValues(s.Name, partitionID, rackID).Set(lastSyncDurationMs)
		// FIXME:
		// ch <- prometheus.NewMetricWithTimestamp(lastSync, prometheus.MustNewConstMetric(collector.switchSyncDurationsMs, prometheus.GaugeValue, lastSyncDurationMs, s.Name, partitionID, rackID))

		for _, c := range s.Connections {
			switchInterfaceInfo.WithLabelValues(s.Name, pointer.SafeDeref(pointer.SafeDeref(c.Nic).Name), c.MachineID, partitionID).Set(1.0)
		}
	}

	return nil
}

var (
	machineAllocationInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_allocation_info",
		Help: "Provide information about the machine allocation",
	}, []string{"machineid", "partition", "machinename", "clusterTag", "primaryASN", "role", "state"})
	machineIssuesInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_issues_info",
		Help: "Provide general information on issues that are evaluated by the metal-api",
	}, []string{"issueid", "description", "severity", "refurl"})
	machineIssues = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_issues",
		Help: "Provide information on machine issues",
	}, []string{"machineid", "issueid"})
	machinePowerUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_power_usage",
		Help: "Provide information about the machine power usage in watts",
	}, []string{"machineid"})
	machinePowerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_power_state",
		Help: "Provide information about the machine power state",
	}, []string{"machineid"})
	machinePowerSuppliesTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_power_supplies_total",
		Help: "Provide information about the total number of power supplies",
	}, []string{"machineid"})
	machinePowerSuppliesHealthy = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_power_supplies_healthy",
		Help: "Provide information about the number of healthy power supplies",
	}, []string{"machineid"})
	machineHardwareInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metal_machine_hardware_info",
		Help: "Provide information about the machine",
	}, []string{"machineid", "partition", "size", "bmcVersion", "biosVersion", "chassisPartNumber", "chassisPartSerial", "boardMfg", "boardMfgSerial",
		"boardPartNumber", "productManufacturer", "productPartNumber", "productSerial"})
)

func machineMetrics(ctx context.Context) error {
	machines, err := client.Machine().FindIPMIMachines(machine.NewFindIPMIMachinesParams().WithBody(&models.V1MachineFindRequest{}).WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving machines: %w", err)
	}

	allIssues, err := client.Machine().ListIssues(machine.NewListIssuesParams().WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving machine issues list: %w", err)
	}

	issues, err := client.Machine().Issues(machine.NewIssuesParams().WithBody(&models.V1MachineIssuesRequest{
		LastErrorThreshold: pointer.Pointer(int64(1 * time.Hour)),
	}).WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving machine issues: %w", err)
	}

	issuesByMachineID := map[string][]string{}
	for _, issue := range issues.Payload {
		issue := issue

		if issue.Machineid == nil {
			continue
		}

		issuesByMachineID[*issue.Machineid] = issue.Issues
	}

	allIssuesByID := map[string]bool{}
	for _, issue := range allIssues.Payload {
		issue := issue

		if issue.ID == nil {
			continue
		}

		allIssuesByID[*issue.ID] = true

		machineIssuesInfo.WithLabelValues(*issue.ID, pointer.SafeDeref(issue.Description), pointer.SafeDeref(issue.Severity), pointer.SafeDeref(issue.RefURL)).Set(1.0)
	}

	for _, m := range machines.Payload {
		m := m

		if m.ID == nil {
			continue
		}

		var (
			partitionID = ""
			role        = ""
			hostname    = "NOTALLOCATED"
			clusterID   = "NOTALLOCATED"
			primaryASN  = "NOTALLOCATED"
			state       = "AVAILABLE"
		)

		if m.State != nil && m.State.Value != nil && *m.State.Value != "" {
			state = *m.State.Value
		}

		if m.Allocation != nil {
			if m.Allocation.Role != nil {
				role = *m.Allocation.Role
			}

			if m.Allocation.Hostname != nil {
				hostname = *m.Allocation.Hostname
			}

			tm := metaltag.NewTagMap(m.Tags)
			if id, ok := tm.Value(metaltag.ClusterID); ok {
				clusterID = id
			}
			if asn, ok := tm.Value(metaltag.MachineNetworkPrimaryASN); ok {
				primaryASN = asn
			}
		}
		if m.Partition != nil && m.Partition.ID != nil {
			partitionID = *m.Partition.ID
		}

		if m.Ipmi != nil {
			if m.Ipmi.Powerstate != nil {
				var powerstate float64
				switch *m.Ipmi.Powerstate {
				case "ON":
					powerstate = 1
				case "OFF":
					powerstate = 0
				default:
					powerstate = -1
				}

				machinePowerState.WithLabelValues(*m.ID).Set(powerstate)
			}

			if m.Ipmi.Powersupplies != nil {
				machinePowerSuppliesTotal.WithLabelValues(*m.ID).Set(float64(len(m.Ipmi.Powersupplies)))

				healthy := 0
				for _, ps := range m.Ipmi.Powersupplies {
					if ps.Status != nil && ps.Status.Health != nil && *ps.Status.Health == "OK" {
						healthy++
					}
				}

				machinePowerSuppliesHealthy.WithLabelValues(*m.ID).Set(float64(healthy))
			}

			if m.Ipmi.Powermetric != nil && m.Ipmi.Powermetric.Averageconsumedwatts != nil {
				machinePowerUsage.WithLabelValues(*m.ID).Set(float64(pointer.SafeDeref(m.Ipmi.Powermetric.Averageconsumedwatts)))
			}

			size := "UNKNOWN"
			if m.Size != nil {
				size = m.Size.Name
			}

			if m.Bios != nil && m.Ipmi.Fru != nil {
				machineHardwareInfo.WithLabelValues(*m.ID, partitionID, size, pointer.SafeDeref(m.Ipmi.Bmcversion),
					pointer.SafeDeref(m.Bios.Version), m.Ipmi.Fru.ChassisPartNumber, m.Ipmi.Fru.ChassisPartSerial, m.Ipmi.Fru.BoardMfg, m.Ipmi.Fru.BoardMfgSerial, m.Ipmi.Fru.BoardPartNumber,
					m.Ipmi.Fru.ProductManufacturer, m.Ipmi.Fru.ProductPartNumber, m.Ipmi.Fru.ProductSerial).Set(1.0)
			}
		}

		machineAllocationInfo.WithLabelValues(*m.ID, partitionID, hostname, clusterID, primaryASN, role, state).Set(1.0)

		for issueID := range allIssuesByID {
			issues, ok := issuesByMachineID[*m.ID]
			if !ok {
				machineIssues.WithLabelValues(*m.ID, issueID).Set(0.0)
				continue
			}

			if slices.Contains(issues, issueID) {
				machineIssues.WithLabelValues(*m.ID, issueID).Set(1.0)
			} else {
				machineIssues.WithLabelValues(*m.ID, issueID).Set(0.0)
			}
		}
	}

	return nil
}
