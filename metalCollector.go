package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	metalgo "github.com/metal-stack/metal-go"
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
)

type metalCollector struct {
	networkInfo       *prometheus.Desc
	usedIps           *prometheus.Desc
	availableIps      *prometheus.Desc
	usedPrefixes      *prometheus.Desc
	availablePrefixes *prometheus.Desc

	capacityTotal             *prometheus.Desc
	capacityWaiting           *prometheus.Desc
	capacityFree              *prometheus.Desc
	capacityAllocatable       *prometheus.Desc
	capacityPhonedHome        *prometheus.Desc
	capacityUnavailable       *prometheus.Desc
	capacityOther             *prometheus.Desc
	capacityAllocated         *prometheus.Desc
	capacityFaulty            *prometheus.Desc
	capacityReservationsTotal *prometheus.Desc
	capacityReservationsUsed  *prometheus.Desc
	usedImage                 *prometheus.Desc

	switchInfo            *prometheus.Desc
	switchInterfaceInfo   *prometheus.Desc
	switchMetalCoreUp     *prometheus.Desc
	switchSyncFailed      *prometheus.Desc
	switchSyncDurationsMs *prometheus.Desc

	machineAllocationInfo *prometheus.Desc
	machineIssues         *prometheus.Desc
	machineIssuesInfo     *prometheus.Desc

	machineIpmiIpAddress        *prometheus.Desc
	machineIpmiLastUpdated      *prometheus.Desc
	machinePowerUsage           *prometheus.Desc
	machinePowerState           *prometheus.Desc
	machinePowerSuppliesTotal   *prometheus.Desc
	machinePowerSuppliesHealthy *prometheus.Desc
	machineHardwareInfo         *prometheus.Desc

	projectInfo *prometheus.Desc

	client metalgo.Client
}

func newMetalCollector(client metalgo.Client) *metalCollector {
	return &metalCollector{
		networkInfo: prometheus.NewDesc("metal_network_info",
			"Provide information about the network",
			[]string{"networkId", "name", "projectId", "description", "partition", "vrf", "prefixes", "destPrefixes", "parentNetworkID", "isPrivateSuper", "useNat", "isUnderlay"}, nil,
		),
		usedIps: prometheus.NewDesc(
			"metal_network_ip_used",
			"The total number of used IPs of the network",
			[]string{"networkId"}, nil,
		),
		availableIps: prometheus.NewDesc(
			"metal_network_ip_available",
			"The total number of available IPs of the network",
			[]string{"networkId"}, nil,
		),
		usedPrefixes: prometheus.NewDesc(
			"metal_network_prefix_used",
			"The total number of used prefixes of the network",
			[]string{"networkId"}, nil,
		),
		availablePrefixes: prometheus.NewDesc(
			"metal_network_prefix_available",
			"The total number of available prefixes of the network",
			[]string{"networkId"}, nil,
		),
		capacityTotal: prometheus.NewDesc(
			"metal_partition_capacity_total",
			"The total number of machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityWaiting: prometheus.NewDesc(
			"metal_partition_capacity_waiting",
			"The total number of waiting machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityFree: prometheus.NewDesc(
			"metal_partition_capacity_free",
			"(DEPRECATED) The total number of allocatable machines in the partition, use metal_partition_capacity_allocatable",
			[]string{"partition", "size"}, nil,
		),
		capacityAllocatable: prometheus.NewDesc(
			"metal_partition_capacity_allocatable",
			"The total number of waiting allocatable machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityAllocated: prometheus.NewDesc(
			"metal_partition_capacity_allocated",
			"The capacity of allocated machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityReservationsTotal: prometheus.NewDesc(
			"metal_partition_capacity_reservations_total",
			"The sum of capacity reservations in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityReservationsUsed: prometheus.NewDesc(
			"metal_partition_capacity_reservations_used",
			"The sum of used capacity reservations in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityPhonedHome: prometheus.NewDesc(
			"metal_partition_capacity_phoned_home",
			"The total number of faulty machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityFaulty: prometheus.NewDesc(
			"metal_partition_capacity_faulty",
			"The capacity of faulty machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityUnavailable: prometheus.NewDesc(
			"metal_partition_capacity_unavailable",
			"The total number of faulty machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityOther: prometheus.NewDesc(
			"metal_partition_capacity_other",
			"The total number of machines in an other state in the partition",
			[]string{"partition", "size"}, nil,
		),
		usedImage: prometheus.NewDesc(
			"metal_image_used_total",
			"The total number of machines using a image",
			[]string{"imageID", "name", "classification", "created", "expirationDate", "features"}, nil,
		),
		switchInfo: prometheus.NewDesc(
			"metal_switch_info",
			"Provide information about the switch",
			[]string{"switchname", "partition", "rackid", "metalCoreVersion", "osVendor", "osVersion", "managementIP"}, nil,
		),
		switchInterfaceInfo: prometheus.NewDesc(
			"metal_switch_interface_info",
			"Provide information about the network",
			[]string{"switchname", "device", "machineid", "partition"}, nil,
		),
		switchMetalCoreUp: prometheus.NewDesc(
			"metal_switch_metal_core_up",
			"1 when the metal-core is up, otherwise 0",
			[]string{"switchname", "partition", "rackid"}, nil,
		),
		switchSyncFailed: prometheus.NewDesc(
			"metal_switch_sync_failed",
			"1 when the switch sync is failing, otherwise 0",
			[]string{"switchname", "partition", "rackid"}, nil,
		),
		switchSyncDurationsMs: prometheus.NewDesc(
			"metal_switch_sync_durations_ms",
			"The duration of the syncs in milliseconds",
			[]string{"switchname", "partition", "rackid"}, nil,
		),
		machineAllocationInfo: prometheus.NewDesc(
			"metal_machine_allocation_info",
			"Provide information about the machine allocation",
			[]string{"machineid", "partition", "machinename", "clusterTag", "primaryASN", "role", "state"}, nil,
		),
		machineIssuesInfo: prometheus.NewDesc(
			"metal_machine_issues_info",
			"Provide general information on issues that are evaluated by the metal-api",
			[]string{"issueid", "description", "severity", "refurl"}, nil,
		),
		machineIssues: prometheus.NewDesc(
			"metal_machine_issues",
			"Provide information on machine issues",
			[]string{"machineid", "issueid"}, nil,
		),
		machineIpmiIpAddress: prometheus.NewDesc(
			"metal_machine_ipmi_address",
			"Provide the ipmi ip address",
			[]string{"machineid", "ipmiIP"}, nil,
		),
		machineIpmiLastUpdated: prometheus.NewDesc(
			"metal_machine_last_updated",
			"Provide the timestamp of the last ipmi update",
			[]string{"machineid"}, nil,
		),
		machinePowerUsage: prometheus.NewDesc(
			"metal_machine_power_usage",
			"Provide information about the machine power usage in watts",
			[]string{"machineid"}, nil,
		),
		machinePowerState: prometheus.NewDesc(
			"metal_machine_power_state",
			"Provide information about the machine power state",
			[]string{"machineid"}, nil,
		),
		machinePowerSuppliesTotal: prometheus.NewDesc(
			"metal_machine_power_supplies_total",
			"Provide information about the total number of power supplies",
			[]string{"machineid"}, nil,
		),
		machinePowerSuppliesHealthy: prometheus.NewDesc(
			"metal_machine_power_supplies_healthy",
			"Provide information about the number of healthy power supplies",
			[]string{"machineid"}, nil,
		),
		machineHardwareInfo: prometheus.NewDesc(
			"metal_machine_hardware_info",
			"Provide information about the machine",
			[]string{"machineid", "partition", "size", "bmcVersion", "biosVersion", "chassisPartNumber", "chassisPartSerial", "boardMfg", "boardMfgSerial",
				"boardPartNumber", "productManufacturer", "productPartNumber", "productSerial"}, nil,
		),
		projectInfo: prometheus.NewDesc(
			"metal_project_info",
			"Provide information about metal projects",
			[]string{"projectId", "name", "tenantId"}, nil,
		),

		client: client,
	}
}

func (collector *metalCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.networkInfo
	ch <- collector.usedIps
	ch <- collector.availableIps
	ch <- collector.usedPrefixes
	ch <- collector.availablePrefixes
	ch <- collector.capacityTotal
	ch <- collector.capacityFree
	ch <- collector.capacityAllocatable
	ch <- collector.capacityPhonedHome
	ch <- collector.capacityUnavailable
	ch <- collector.capacityOther
	ch <- collector.capacityWaiting
	ch <- collector.capacityAllocated
	ch <- collector.capacityFaulty
	ch <- collector.capacityReservationsTotal
	ch <- collector.capacityReservationsUsed
	ch <- collector.usedImage
	ch <- collector.switchInfo
	ch <- collector.switchInterfaceInfo
	ch <- collector.switchMetalCoreUp
	ch <- collector.switchSyncFailed
	ch <- collector.switchSyncDurationsMs
	ch <- collector.machineAllocationInfo
	ch <- collector.machineIssues
	ch <- collector.machineIssuesInfo
	ch <- collector.machineIpmiIpAddress
	ch <- collector.machineIpmiLastUpdated
	ch <- collector.machinePowerUsage
	ch <- collector.machinePowerState
	ch <- collector.machinePowerSuppliesTotal
	ch <- collector.machinePowerSuppliesHealthy
}

func (collector *metalCollector) Collect(ch chan<- prometheus.Metric) {
	networks, err := collector.client.Network().ListNetworks(network.NewListNetworksParams(), nil)
	if err != nil {
		panic(err)
	}
	for _, n := range networks.Payload {
		if n.Privatesuper == nil || n.Nat == nil || n.Underlay == nil || n.ID == nil {
			continue
		}
		privateSuper := fmt.Sprintf("%t", *n.Privatesuper)
		nat := fmt.Sprintf("%t", *n.Nat)
		underlay := fmt.Sprintf("%t", *n.Underlay)
		prefixes := strings.Join(n.Prefixes, ",")
		destPrefixes := strings.Join(n.Destinationprefixes, ",")
		vrf := fmt.Sprintf("%d", n.Vrf)

		// {"networkId", "name", "projectId", "description", "partition", "vrf", "prefixes", "destPrefixes", "parentNetworkID", "isPrivateSuper", "useNat", "isUnderlay"}, nil,
		ch <- prometheus.MustNewConstMetric(collector.networkInfo, prometheus.GaugeValue, 1.0, *n.ID, n.Name, n.Projectid, n.Description, n.Partitionid, vrf, prefixes, destPrefixes, n.Parentnetworkid, privateSuper, nat, underlay)
		if n.Usage == nil {
			continue
		}
		ch <- prometheus.MustNewConstMetric(collector.usedIps, prometheus.GaugeValue, float64(pointer.SafeDeref(n.Usage.UsedIps)), *n.ID)
		ch <- prometheus.MustNewConstMetric(collector.availableIps, prometheus.GaugeValue, float64(pointer.SafeDeref(n.Usage.AvailableIps)), *n.ID)
		ch <- prometheus.MustNewConstMetric(collector.usedPrefixes, prometheus.GaugeValue, float64(pointer.SafeDeref(n.Usage.UsedPrefixes)), *n.ID)
		ch <- prometheus.MustNewConstMetric(collector.availablePrefixes, prometheus.GaugeValue, float64(pointer.SafeDeref(n.Usage.AvailablePrefixes)), *n.ID)
	}

	caps, err := collector.client.Partition().PartitionCapacity(partition.NewPartitionCapacityParams().WithBody(&models.V1PartitionCapacityRequest{}), nil)
	if err != nil {
		panic(err)
	}
	for _, p := range caps.Payload {
		if p.ID == nil {
			continue
		}
		for _, s := range p.Servers {
			if s.Size == nil {
				continue
			}
			ch <- prometheus.MustNewConstMetric(collector.capacityTotal, prometheus.GaugeValue, float64(s.Total), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityAllocated, prometheus.GaugeValue, float64(s.Allocated), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityWaiting, prometheus.GaugeValue, float64(s.Waiting), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityFree, prometheus.GaugeValue, float64(s.Allocatable), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityAllocatable, prometheus.GaugeValue, float64(s.Allocatable), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityFaulty, prometheus.GaugeValue, float64(s.Faulty), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityReservationsTotal, prometheus.GaugeValue, float64(s.Reservations), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityReservationsUsed, prometheus.GaugeValue, float64(s.Usedreservations), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityPhonedHome, prometheus.GaugeValue, float64(s.PhonedHome), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityUnavailable, prometheus.GaugeValue, float64(s.Unavailable), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityOther, prometheus.GaugeValue, float64(s.Other), *p.ID, *s.Size)
		}
	}

	imageParams := image.NewListImagesParams()
	imageParams.SetShowUsage(pointer.Pointer(true))
	images, err := collector.client.Image().ListImages(imageParams, nil)
	if err != nil {
		panic(err)
	}
	for _, i := range images.Payload {
		if i.ID == nil {
			continue
		}
		usage := len(i.Usedby)
		created := fmt.Sprintf("%d", time.Time(i.Created).Unix())
		expirationDate := ""
		if i.ExpirationDate != nil {
			expirationDate = fmt.Sprintf("%d", time.Time(*i.ExpirationDate).Unix())
		}
		features := strings.Join(i.Features, ",")
		ch <- prometheus.MustNewConstMetric(collector.usedImage, prometheus.GaugeValue, float64(usage), *i.ID, i.Name, i.Classification, created, expirationDate, features)
	}

	switches, err := collector.client.SwitchOperations().ListSwitches(switch_operations.NewListSwitchesParams(), nil)
	if err != nil {
		panic(err)
	}
	for _, s := range switches.Payload {
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

		ch <- prometheus.MustNewConstMetric(collector.switchMetalCoreUp, prometheus.GaugeValue, metalCoreUp, s.Name, partitionID, rackID)
		ch <- prometheus.MustNewConstMetric(collector.switchSyncFailed, prometheus.GaugeValue, syncFailed, s.Name, partitionID, rackID)
		ch <- prometheus.NewMetricWithTimestamp(lastSync, prometheus.MustNewConstMetric(collector.switchSyncDurationsMs, prometheus.GaugeValue, lastSyncDurationMs, s.Name, partitionID, rackID))
		ch <- prometheus.MustNewConstMetric(collector.switchInfo, prometheus.GaugeValue, 1.0, s.Name, partitionID, rackID, metalCoreVersion, osVendor, osVersion, managementIP)

		for _, c := range s.Connections {
			ch <- prometheus.MustNewConstMetric(collector.switchInterfaceInfo, prometheus.GaugeValue, 1.0, s.Name, pointer.SafeDeref(pointer.SafeDeref(c.Nic).Name), c.MachineID, partitionID)
		}
	}

	machines, err := collector.client.Machine().FindIPMIMachines(machine.NewFindIPMIMachinesParams().WithBody(&models.V1MachineFindRequest{}), nil)
	if err != nil {
		panic(err)
	}

	allIssues, err := collector.client.Machine().ListIssues(machine.NewListIssuesParams(), nil)
	if err != nil {
		panic(err)
	}

	issues, err := collector.client.Machine().Issues(machine.NewIssuesParams().WithBody(&models.V1MachineIssuesRequest{
		LastErrorThreshold: pointer.Pointer(int64(1 * time.Hour)),
	}), nil)
	if err != nil {
		panic(err)
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

		ch <- prometheus.MustNewConstMetric(collector.machineIssuesInfo, prometheus.GaugeValue, 1.0, *issue.ID, pointer.SafeDeref(issue.Description), pointer.SafeDeref(issue.Severity), pointer.SafeDeref(issue.RefURL))
	}

	ipmiIPs := map[string]string{}
	ipmiLastUpdates := map[string]time.Time{}

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
				ch <- prometheus.MustNewConstMetric(collector.machinePowerState, prometheus.GaugeValue, powerstate, *m.ID)
			}
			if m.Ipmi.Powersupplies != nil {
				ch <- prometheus.MustNewConstMetric(collector.machinePowerSuppliesTotal, prometheus.GaugeValue, float64(len(m.Ipmi.Powersupplies)), *m.ID)
				healthy := 0
				for _, ps := range m.Ipmi.Powersupplies {
					if ps.Status != nil && ps.Status.Health != nil && *ps.Status.Health == "OK" {
						healthy++
					}
				}
				ch <- prometheus.MustNewConstMetric(collector.machinePowerSuppliesHealthy, prometheus.GaugeValue, float64(healthy), *m.ID)
			}
			if m.Ipmi.LastUpdated != nil {
				lu := time.Time(*m.Ipmi.LastUpdated)
				ch <- prometheus.MustNewConstMetric(collector.machineIpmiLastUpdated, prometheus.GaugeValue, float64(lu.Unix()), *m.ID)
				if m.Ipmi.Address != nil {
					// only store the latest ip address to prevent duplicate ipmiIP labels
					if t, ok := ipmiLastUpdates[*m.ID]; !ok || lu.After(t) {
						ipmiLastUpdates[*m.ID] = lu
						ipmiIPs[*m.ID] = *m.Ipmi.Address
					}
				}
			}
			if m.Ipmi.Powermetric != nil && m.Ipmi.Powermetric.Averageconsumedwatts != nil {
				ch <- prometheus.MustNewConstMetric(collector.machinePowerUsage, prometheus.GaugeValue, float64(pointer.SafeDeref(m.Ipmi.Powermetric.Averageconsumedwatts)), *m.ID)
			}
			size := "UNKNOWN"
			if m.Size != nil {
				size = m.Size.Name
			}
			if m.Bios != nil && m.Ipmi.Fru != nil {
				ch <- prometheus.MustNewConstMetric(collector.machineHardwareInfo, prometheus.GaugeValue, 1.0, *m.ID, partitionID, size, pointer.SafeDeref(m.Ipmi.Bmcversion),
					pointer.SafeDeref(m.Bios.Version), m.Ipmi.Fru.ChassisPartNumber, m.Ipmi.Fru.ChassisPartSerial, m.Ipmi.Fru.BoardMfg, m.Ipmi.Fru.BoardMfgSerial, m.Ipmi.Fru.BoardPartNumber,
					m.Ipmi.Fru.ProductManufacturer, m.Ipmi.Fru.ProductPartNumber, m.Ipmi.Fru.ProductSerial)
			}
		}
		ch <- prometheus.MustNewConstMetric(collector.machineAllocationInfo, prometheus.GaugeValue, 1.0, *m.ID, partitionID, hostname, clusterID, primaryASN, role, state)

		for issueID := range allIssuesByID {
			machineIssues, ok := issuesByMachineID[*m.ID]
			if !ok {
				ch <- prometheus.MustNewConstMetric(collector.machineIssues, prometheus.GaugeValue, 0.0, *m.ID, issueID)
				continue
			}

			if slices.Contains(machineIssues, issueID) {
				ch <- prometheus.MustNewConstMetric(collector.machineIssues, prometheus.GaugeValue, 1.0, *m.ID, issueID)
			} else {
				ch <- prometheus.MustNewConstMetric(collector.machineIssues, prometheus.GaugeValue, 0.0, *m.ID, issueID)
			}
		}
	}

	for mId, ip := range ipmiIPs {
		ch <- prometheus.MustNewConstMetric(collector.machineIpmiIpAddress, prometheus.GaugeValue, 1.0, mId, ip)
	}

	projects, err := collector.client.Project().ListProjects(project.NewListProjectsParams(), nil)
	if err != nil {
		panic(err)
	}

	for _, p := range projects.Payload {
		ch <- prometheus.MustNewConstMetric(collector.projectInfo, prometheus.GaugeValue, 1.0, p.Meta.ID, p.Name, p.TenantID)
	}
}
