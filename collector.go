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
	"golang.org/x/sync/errgroup"
)

type collector struct{}

var (
	metrics = &GenSyncMap[*prometheus.Desc, prometheus.Metric]{}

	descs = []*prometheus.Desc{
		metalImageUsedTotal,
		metalNetworkInfo,
		metalNetworkUsedIPs,
		metalNetworkAvailableIps,
		metalNetworkUsedPrefixes,
		metalNetworkAvailablePrefixes,
		metalProjectInfo,
		metalSwitchInfo,
		metalSwitchInterfaceInfo,
		metalSwitchMetalCoreUp,
		metalSwitchSyncFailed,
		metalSwitchSyncDurationsMs,
		metalMachineAllocationInfo,
		metalMachineIssuesInfo,
		metalMachineIssues,
		metalMachinePowerUsage,
		metalMachinePowerState,
		metalMachinePowerSuppliesTotal,
		metalMachinePowerSuppliesHealthy,
		metalMachineHardwareInfo,
		metalPartitionCapacityTotal,
		metalPartitionCapacityWaiting,
		metalPartitionCapacityFree,
		metalPartitionCapacityAllocatable,
		metalPartitionCapacityAllocated,
		metalPartitionCapacityReservationsTotal,
		metalPartitionCapacityReservationsUsed,
		metalPartitionCapacityPhonedHome,
		metalPartitionCapacityFaulty,
		metalPartitionCapacityUnavailable,
		metalPartitionCapacityOther,
	}

	// image
	metalImageUsedTotal = prometheus.NewDesc(
		"metal_image_used_total",
		"The total number of machines using a image",
		[]string{"imageID", "name", "classification", "created", "expirationDate", "features"},
		nil,
	)

	// network
	metalNetworkInfo = prometheus.NewDesc(
		"metal_network_info",
		"Provide information about the network",
		[]string{"networkId", "name", "projectId", "description", "partition", "vrf", "prefixes", "destPrefixes", "parentNetworkID", "isPrivateSuper", "useNat", "isUnderlay"},
		nil,
	)
	metalNetworkUsedIPs = prometheus.NewDesc(
		"metal_network_ip_used",
		"The total number of used IPs of the network",
		[]string{"networkId"},
		nil,
	)
	metalNetworkAvailableIps = prometheus.NewDesc(
		"metal_network_ip_available",
		"The total number of available IPs of the network",
		[]string{"networkId"},
		nil,
	)
	metalNetworkUsedPrefixes = prometheus.NewDesc(
		"metal_network_prefix_used",
		"The total number of used prefixes of the network",
		[]string{"networkId"},
		nil,
	)
	metalNetworkAvailablePrefixes = prometheus.NewDesc(
		"metal_network_prefix_available",
		"The total number of available prefixes of the network",
		[]string{"networkId"},
		nil,
	)

	// partition
	metalPartitionCapacityTotal = prometheus.NewDesc(
		"metal_partition_capacity_total",
		"The total number of machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityWaiting = prometheus.NewDesc(
		"metal_partition_capacity_waiting",
		"The total number of waiting machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityFree = prometheus.NewDesc(
		"metal_partition_capacity_free",
		"(DEPRECATED) The total number of allocatable machines in the partition, use metal_partition_capacity_allocatable",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityAllocatable = prometheus.NewDesc(
		"metal_partition_capacity_allocatable",
		"The total number of waiting allocatable machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityAllocated = prometheus.NewDesc(
		"metal_partition_capacity_allocated",
		"The capacity of allocated machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityReservationsTotal = prometheus.NewDesc(
		"metal_partition_capacity_reservations_total",
		"The sum of capacity reservations in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityReservationsUsed = prometheus.NewDesc(
		"metal_partition_capacity_reservations_used",
		"The sum of used capacity reservations in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityPhonedHome = prometheus.NewDesc(
		"metal_partition_capacity_phoned_home",
		"The total number of faulty machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityFaulty = prometheus.NewDesc(
		"metal_partition_capacity_faulty",
		"The capacity of faulty machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityUnavailable = prometheus.NewDesc(
		"metal_partition_capacity_unavailable",
		"The total number of unavailable machines in the partition",
		[]string{"partition", "size"},
		nil,
	)
	metalPartitionCapacityOther = prometheus.NewDesc(
		"metal_partition_capacity_other",
		"The total number of machines in an other state in the partition",
		[]string{"partition", "size"},
		nil,
	)

	// project
	metalProjectInfo = prometheus.NewDesc(
		"metal_project_info",
		"Provide information about metal projects",
		[]string{"projectId", "name", "tenantId"},
		nil,
	)

	// switch
	metalSwitchInfo = prometheus.NewDesc(
		"metal_switch_info",
		"Provide information about the switch",
		[]string{"switchname", "partition", "rackid", "metalCoreVersion", "osVendor", "osVersion", "managementIP"},
		nil,
	)
	metalSwitchInterfaceInfo = prometheus.NewDesc(
		"metal_switch_interface_info",
		"Provide information about the switch interfaces",
		[]string{"switchname", "device", "machineid", "partition"},
		nil,
	)
	metalSwitchMetalCoreUp = prometheus.NewDesc(
		"metal_switch_metal_core_up",
		"1 when the metal-core is up, otherwise 0",
		[]string{"switchname", "partition", "rackid"},
		nil,
	)
	metalSwitchSyncFailed = prometheus.NewDesc(
		"metal_switch_sync_failed",
		"1 when the switch sync is failing, otherwise 0",
		[]string{"switchname", "partition", "rackid"},
		nil,
	)
	metalSwitchSyncDurationsMs = prometheus.NewDesc(
		"metal_switch_sync_durations_ms",
		"The duration of the syncs in milliseconds",
		[]string{"switchname", "partition", "rackid"},
		nil,
	)

	// machine
	metalMachineAllocationInfo = prometheus.NewDesc(
		"metal_machine_allocation_info",
		"Provide information about the machine allocation",
		[]string{"machineid", "partition", "machinename", "clusterTag", "primaryASN", "role", "state"},
		nil,
	)
	metalMachineIssuesInfo = prometheus.NewDesc(
		"metal_machine_issues_info",
		"Provide general information on issues that are evaluated by the metal-api",
		[]string{"issueid", "description", "severity", "refurl"},
		nil,
	)
	metalMachineIssues = prometheus.NewDesc(
		"metal_machine_issues",
		"Provide information on machine issues",
		[]string{"machineid", "issueid"},
		nil,
	)
	metalMachinePowerUsage = prometheus.NewDesc(
		"metal_machine_power_usage",
		"Provide information about the machine power usage in watts",
		[]string{"machineid"},
		nil,
	)
	metalMachinePowerState = prometheus.NewDesc(
		"metal_machine_power_state",
		"Provide information about the machine power state",
		[]string{"machineid"},
		nil,
	)
	metalMachinePowerSuppliesTotal = prometheus.NewDesc(
		"metal_machine_power_supplies_total",
		"Provide information about the total number of power supplies",
		[]string{"machineid"},
		nil,
	)
	metalMachinePowerSuppliesHealthy = prometheus.NewDesc(
		"metal_machine_power_supplies_healthy",
		"Provide information about the number of healthy power supplies",
		[]string{"machineid"},
		nil,
	)
	metalMachineHardwareInfo = prometheus.NewDesc(
		"metal_machine_hardware_info",
		"Provide information about the machine",
		[]string{"machineid", "partition", "size", "bmcVersion", "biosVersion", "chassisPartNumber", "chassisPartSerial", "boardMfg", "boardMfgSerial",
			"boardPartNumber", "productManufacturer", "productPartNumber", "productSerial"},
		nil,
	)
)

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range descs {
		ch <- desc
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	metrics.Range(func(_ *prometheus.Desc, m prometheus.Metric) bool {
		ch <- m
		return true
	})
}

func storeGauge(desc *prometheus.Desc, value float64, lvs ...string) {
	m := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, lvs...)
	metrics.Store(desc, m)
}

func storeGaugeTimestamp(ts time.Time, desc *prometheus.Desc, value float64, lvs ...string) {
	m := prometheus.NewMetricWithTimestamp(ts, prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, lvs...))
	metrics.Store(desc, m)
}

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

		storeGauge(metalNetworkInfo, 1.0, nwID, nw.Name, nw.Projectid, nw.Description, nw.Partitionid, vrf, prefixes, destPrefixes, nw.Parentnetworkid, privateSuper, nat, underlay)

		if nw.Usage == nil {
			continue
		}

		storeGauge(metalNetworkUsedIPs, float64(pointer.SafeDeref(nw.Usage.UsedIps)), nwID)
		storeGauge(metalNetworkAvailableIps, float64(pointer.SafeDeref(nw.Usage.AvailableIps)), nwID)
		storeGauge(metalNetworkUsedPrefixes, float64(pointer.SafeDeref(nw.Usage.UsedPrefixes)), nwID)
		storeGauge(metalNetworkAvailablePrefixes, float64(pointer.SafeDeref(nw.Usage.AvailablePrefixes)), nwID)
	}

	return nil
}

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

			storeGauge(metalPartitionCapacityTotal, float64(s.Total), pID, size)
			storeGauge(metalPartitionCapacityAllocated, float64(s.Allocated), pID, size)
			storeGauge(metalPartitionCapacityWaiting, float64(s.Waiting), pID, size)
			storeGauge(metalPartitionCapacityFree, float64(s.Allocatable), pID, size)
			storeGauge(metalPartitionCapacityAllocatable, float64(s.Allocatable), pID, size)
			storeGauge(metalPartitionCapacityFaulty, float64(s.Faulty), pID, size)
			storeGauge(metalPartitionCapacityReservationsTotal, float64(s.Reservations), pID, size)
			storeGauge(metalPartitionCapacityReservationsUsed, float64(s.Usedreservations), pID, size)
			storeGauge(metalPartitionCapacityPhonedHome, float64(s.PhonedHome), pID, size)
			storeGauge(metalPartitionCapacityUnavailable, float64(s.Unavailable), pID, size)
			storeGauge(metalPartitionCapacityOther, float64(s.Other), pID, size)
		}
	}

	return nil
}

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

		storeGauge(metalImageUsedTotal, float64(usage), id, i.Name, i.Classification, created, expirationDate, features)
	}

	return nil
}

func projectMetrics(ctx context.Context) error {
	resp, err := client.Project().ListProjects(project.NewListProjectsParams().WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("error retrieving images: %w", err)
	}

	for _, p := range resp.Payload {
		storeGauge(metalProjectInfo, float64(1.0), p.Meta.ID, p.Name, p.TenantID)
	}

	return nil
}

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

		storeGauge(metalSwitchInfo, 1.0, s.Name, partitionID, rackID, metalCoreVersion, osVendor, osVersion, managementIP)
		storeGauge(metalSwitchMetalCoreUp, metalCoreUp, s.Name, partitionID, rackID)
		storeGauge(metalSwitchSyncFailed, syncFailed, s.Name, partitionID, rackID)
		storeGaugeTimestamp(lastSync, metalSwitchSyncDurationsMs, lastSyncDurationMs, s.Name, partitionID, rackID)

		for _, c := range s.Connections {
			storeGauge(metalSwitchInterfaceInfo, 1.0, s.Name, pointer.SafeDeref(pointer.SafeDeref(c.Nic).Name), c.MachineID, partitionID)
		}
	}

	return nil
}

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

		storeGauge(metalMachineIssuesInfo, 1.0, *issue.ID, pointer.SafeDeref(issue.Description), pointer.SafeDeref(issue.Severity), pointer.SafeDeref(issue.RefURL))
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

				storeGauge(metalMachinePowerState, powerstate, *m.ID)
			}

			if m.Ipmi.Powersupplies != nil {
				storeGauge(metalMachinePowerSuppliesTotal, float64(len(m.Ipmi.Powersupplies)), *m.ID)

				healthy := 0
				for _, ps := range m.Ipmi.Powersupplies {
					if ps.Status != nil && ps.Status.Health != nil && *ps.Status.Health == "OK" {
						healthy++
					}
				}

				storeGauge(metalMachinePowerSuppliesHealthy, float64(healthy), *m.ID)
			}

			if m.Ipmi.Powermetric != nil && m.Ipmi.Powermetric.Averageconsumedwatts != nil {
				storeGauge(metalMachinePowerUsage, float64(pointer.SafeDeref(m.Ipmi.Powermetric.Averageconsumedwatts)), *m.ID)
			}

			size := "UNKNOWN"
			if m.Size != nil {
				size = m.Size.Name
			}

			if m.Bios != nil && m.Ipmi.Fru != nil {
				storeGauge(metalMachineHardwareInfo, 1.0, *m.ID, partitionID, size, pointer.SafeDeref(m.Ipmi.Bmcversion),
					pointer.SafeDeref(m.Bios.Version), m.Ipmi.Fru.ChassisPartNumber, m.Ipmi.Fru.ChassisPartSerial, m.Ipmi.Fru.BoardMfg, m.Ipmi.Fru.BoardMfgSerial, m.Ipmi.Fru.BoardPartNumber,
					m.Ipmi.Fru.ProductManufacturer, m.Ipmi.Fru.ProductPartNumber, m.Ipmi.Fru.ProductSerial)
			}
		}

		storeGauge(metalMachineAllocationInfo, 1.0, *m.ID, partitionID, hostname, clusterID, primaryASN, role, state)

		for issueID := range allIssuesByID {
			issues, ok := issuesByMachineID[*m.ID]
			if !ok {
				storeGauge(metalMachineIssues, 1.0, *m.ID, issueID)
				continue
			}

			if slices.Contains(issues, issueID) {
				storeGauge(metalMachineIssues, 1.0, *m.ID, issueID)
			} else {
				storeGauge(metalMachineIssues, 0.0, *m.ID, issueID)
			}
		}
	}

	return nil
}
