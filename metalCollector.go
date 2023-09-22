package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/client/partition"
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

	capacityTotal     *prometheus.Desc
	capacityFree      *prometheus.Desc
	capacityAllocated *prometheus.Desc
	capacityFaulty    *prometheus.Desc
	usedImage         *prometheus.Desc

	switchInfo            *prometheus.Desc
	switchInterfaceInfo   *prometheus.Desc
	switchSyncFailed      *prometheus.Desc
	switchSyncDurationsMs *prometheus.Desc

	machineAllocationInfo *prometheus.Desc

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
			"The total capacity of machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityFree: prometheus.NewDesc(
			"metal_partition_capacity_free",
			"The capacity of free machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityAllocated: prometheus.NewDesc(
			"metal_partition_capacity_allocated",
			"The capacity of allocated machines in the partition",
			[]string{"partition", "size"}, nil,
		),
		capacityFaulty: prometheus.NewDesc(
			"metal_partition_capacity_faulty",
			"The capacity of faulty machines in the partition",
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
			[]string{"machineid", "partition", "machinename", "clusterTag", "primaryASN", "role"}, nil,
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
	ch <- collector.capacityAllocated
	ch <- collector.capacityFaulty
	ch <- collector.usedImage
	ch <- collector.switchInfo
	ch <- collector.switchInterfaceInfo
	ch <- collector.switchSyncFailed
	ch <- collector.switchSyncDurationsMs
	ch <- collector.machineAllocationInfo
}

func (collector *metalCollector) Collect(ch chan<- prometheus.Metric) {
	networks, err := collector.client.Network().ListNetworks(network.NewListNetworksParams(), nil)
	if err != nil {
		panic(err)
	}
	for _, n := range networks.Payload {
		privateSuper := fmt.Sprintf("%t", *n.Privatesuper)
		nat := fmt.Sprintf("%t", *n.Nat)
		underlay := fmt.Sprintf("%t", *n.Underlay)
		prefixes := strings.Join(n.Prefixes, ",")
		destPrefixes := strings.Join(n.Destinationprefixes, ",")
		vrf := strconv.FormatUint(uint64(n.Vrf), 10)

		// {"networkId", "name", "projectId", "description", "partition", "vrf", "prefixes", "destPrefixes", "parentNetworkID", "isPrivateSuper", "useNat", "isUnderlay"}, nil,
		ch <- prometheus.MustNewConstMetric(collector.networkInfo, prometheus.GaugeValue, 1.0, *n.ID, n.Name, n.Projectid, n.Description, n.Partitionid, vrf, prefixes, destPrefixes, n.Parentnetworkid, privateSuper, nat, underlay)
		ch <- prometheus.MustNewConstMetric(collector.usedIps, prometheus.GaugeValue, float64(*n.Usage.UsedIps), *n.ID)
		ch <- prometheus.MustNewConstMetric(collector.availableIps, prometheus.GaugeValue, float64(*n.Usage.AvailableIps), *n.ID)
		ch <- prometheus.MustNewConstMetric(collector.usedPrefixes, prometheus.GaugeValue, float64(*n.Usage.UsedPrefixes), *n.ID)
		ch <- prometheus.MustNewConstMetric(collector.availablePrefixes, prometheus.GaugeValue, float64(*n.Usage.AvailablePrefixes), *n.ID)
	}

	caps, err := collector.client.Partition().PartitionCapacity(partition.NewPartitionCapacityParams().WithBody(&models.V1PartitionCapacityRequest{}), nil)
	if err != nil {
		panic(err)
	}
	for _, p := range caps.Payload {
		for _, s := range p.Servers {
			ch <- prometheus.MustNewConstMetric(collector.capacityTotal, prometheus.GaugeValue, float64(*s.Total), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityAllocated, prometheus.GaugeValue, float64(*s.Allocated), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityFree, prometheus.GaugeValue, float64(*s.Free), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityFaulty, prometheus.GaugeValue, float64(*s.Faulty), *p.ID, *s.Size)
		}
	}

	imageParams := image.NewListImagesParams()
	imageParams.SetShowUsage(pointer.Pointer(true))
	images, err := collector.client.Image().ListImages(imageParams, nil)
	if err != nil {
		panic(err)
	}
	for _, i := range images.Payload {
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
			managementIP     = s.ManagementIP
		)

		if lastSyncError.After(lastSync) {
			syncFailed = 1.0
			lastSyncDurationMs = lastSyncErrorDurationMs
			lastSync = lastSyncError
		}

		ch <- prometheus.MustNewConstMetric(collector.switchSyncFailed, prometheus.GaugeValue, syncFailed, s.Name, partitionID, rackID)
		ch <- prometheus.NewMetricWithTimestamp(lastSync, prometheus.MustNewConstMetric(collector.switchSyncDurationsMs, prometheus.GaugeValue, lastSyncDurationMs, s.Name, partitionID, rackID))
		ch <- prometheus.MustNewConstMetric(collector.switchInfo, prometheus.GaugeValue, 1.0, s.Name, partitionID, rackID, metalCoreVersion, osVendor, osVersion, managementIP)

		for _, c := range s.Connections {
			ch <- prometheus.MustNewConstMetric(collector.switchInterfaceInfo, prometheus.GaugeValue, 1.0, s.Name, pointer.SafeDeref(pointer.SafeDeref(c.Nic).Name), c.MachineID, partitionID)
		}
	}

	machines, err := collector.client.Machine().FindMachines(machine.NewFindMachinesParams().WithBody(&models.V1MachineFindRequest{}), nil)
	if err != nil {
		panic(err)
	}

	for _, m := range machines.Payload {
		if m.ID != nil && m.Allocation != nil && m.Allocation.Hostname != nil {

			clusterTag := ""
			primaryASN := ""
			for _, t := range m.Tags {
				tag, value, found := strings.Cut(t, "=")
				if found {
					switch tag {
					case metaltag.ClusterID:
						clusterTag = value
					case metaltag.MachineNetworkPrimaryASN:
						primaryASN = value
					}
				}
			}

			ch <- prometheus.MustNewConstMetric(collector.machineAllocationInfo, prometheus.GaugeValue, 1.0, *m.ID, *m.Partition.ID, *m.Allocation.Hostname, clusterTag, primaryASN, *m.Allocation.Role)
		} else if m.ID != nil && m.Partition != nil && m.Partition.ID != nil {
			ch <- prometheus.MustNewConstMetric(collector.machineAllocationInfo, prometheus.GaugeValue, 1.0, *m.ID, *m.Partition.ID, "NOTALLOCATED", "NOTALLOCATED", "NOTALLOCATED", "NOTALLOCATED")
		}
	}

}
