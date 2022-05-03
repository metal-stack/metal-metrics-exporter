package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	metalgo "github.com/metal-stack/metal-go"

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

	driver *metalgo.Driver
}

func newMetalCollector(driver *metalgo.Driver) *metalCollector {
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

		driver: driver,
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
}

func (collector *metalCollector) Collect(ch chan<- prometheus.Metric) {
	networks, err := collector.driver.NetworkList()
	if err != nil {
		panic(err)
	}
	for _, n := range networks.Networks {
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

	caps, err := collector.driver.PartitionCapacity(metalgo.PartitionCapacityRequest{})
	if err != nil {
		panic(err)
	}
	for _, p := range caps.Capacity {
		for _, s := range p.Servers {
			ch <- prometheus.MustNewConstMetric(collector.capacityTotal, prometheus.GaugeValue, float64(*s.Total), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityAllocated, prometheus.GaugeValue, float64(*s.Allocated), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityFree, prometheus.GaugeValue, float64(*s.Free), *p.ID, *s.Size)
			ch <- prometheus.MustNewConstMetric(collector.capacityFaulty, prometheus.GaugeValue, float64(*s.Faulty), *p.ID, *s.Size)
		}
	}

	images, err := collector.driver.ImageListWithUsage()
	if err != nil {
		panic(err)
	}
	for _, i := range images.Image {
		usage := len(i.Usedby)
		created := fmt.Sprintf("%d", time.Time(i.Created).Unix())
		expirationDate := ""
		if i.ExpirationDate != nil {
			expirationDate = fmt.Sprintf("%d", time.Time(*i.ExpirationDate).Unix())
		}
		features := strings.Join(i.Features, ",")
		ch <- prometheus.MustNewConstMetric(collector.usedImage, prometheus.GaugeValue, float64(usage), *i.ID, i.Name, i.Classification, created, expirationDate, features)
	}
}
