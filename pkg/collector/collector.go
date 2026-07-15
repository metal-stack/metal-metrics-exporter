package collector

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	apiv2client "github.com/metal-stack/api/go/client"
	"github.com/metal-stack/api/go/enum"
	adminv2 "github.com/metal-stack/api/go/metalstack/admin/v2"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/api/go/tag"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

type collector struct {
	client        apiv2client.Client
	updateTimeout time.Duration

	mu                         sync.Mutex
	newMetrics, currentMetrics []prometheus.Metric
}

var (
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
		switchInterfaceBGPTimeStampEstablished,
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
		[]string{"networkId", "name", "projectId", "description", "partition", "vrf", "prefixes", "destPrefixes", "parentNetworkID", "isPrivateSuper", "useNat", "isUnderlay", "clusterTag"},
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
	switchInterfaceBGPTimeStampEstablished = prometheus.NewDesc(
		"metal_switch_interface_bgp_established_timestamp",
		"Provide the unix timestamp since BGP is established on the interfaces (0 if not established)",
		[]string{"switchname", "device", "machineid", "partition"}, nil,
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
		[]string{"machineid", "partition", "machinename", "clusterTag", "primaryASN", "role", "state", "imageId"},
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

func New(client apiv2client.Client, updateTimeout time.Duration) *collector {
	return &collector{
		client:         client,
		updateTimeout:  updateTimeout,
		mu:             sync.Mutex{},
		newMetrics:     nil,
		currentMetrics: nil,
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range descs {
		ch <- desc
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	for _, m := range c.currentMetrics {
		ch <- m
	}
}

func (c *collector) Update() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.updateTimeout)
	defer cancel()

	c.newMetrics = nil

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(2) // to be graceful with the metal-api

	g.Go(func() error { return c.networkMetrics(ctx) })
	g.Go(func() error { return c.partitionMetrics(ctx) })
	g.Go(func() error { return c.imageMetrics(ctx) })
	g.Go(func() error { return c.projectMetrics(ctx) })
	g.Go(func() error { return c.switchMetrics(ctx) })
	g.Go(func() error { return c.machineMetrics(ctx) })

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error during metrics update: %w", err)
	}

	c.currentMetrics = c.newMetrics

	return nil
}

func (c *collector) storeGauge(desc *prometheus.Desc, value float64, lvs ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, lvs...)
	c.newMetrics = append(c.newMetrics, m)
}

func (c *collector) storeGaugeTimestamp(ts time.Time, desc *prometheus.Desc, value float64, lvs ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := prometheus.NewMetricWithTimestamp(ts, prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, lvs...))
	c.newMetrics = append(c.newMetrics, m)
}

func (c *collector) networkMetrics(ctx context.Context) error {
	resp, err := c.client.Adminv2().Network().List(ctx, &adminv2.NetworkServiceListRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving networks: %w", err)
	}

	for _, nw := range resp.Networks {
		var (
			nwID         = nw.Id
			nat          = nw.NatType == apiv2.NATType_NAT_TYPE_IPV4_MASQUERADE
			underlay     = nw.Type == apiv2.NetworkType_NETWORK_TYPE_UNDERLAY
			prefixes     = strings.Join(nw.Prefixes, ",")
			destPrefixes = strings.Join(nw.DestinationPrefixes, ",")
			vrf          = ""

			isSuperNetwork bool
			clusterId      = ""
		)
		if nw.Vrf != nil {
			vrf = fmt.Sprintf("%d", *nw.Vrf)
		}

		if nw.Meta != nil && nw.Meta.Labels != nil && nw.Meta.Labels.Labels != nil {
			if id, ok := nw.Meta.Labels.Labels[tag.ClusterID]; ok {
				clusterId = id
			}
		}

		if nw.Type == apiv2.NetworkType_NETWORK_TYPE_SUPER || nw.Type == apiv2.NetworkType_NETWORK_TYPE_SUPER_NAMESPACED {
			isSuperNetwork = true
		}

		c.storeGauge(metalNetworkInfo, 1.0, nwID,
			pointer.SafeDeref(nw.Name),
			pointer.SafeDeref(nw.Project),
			pointer.SafeDeref(nw.Description),
			pointer.SafeDeref(nw.Partition),
			vrf,
			prefixes,
			destPrefixes,
			pointer.SafeDeref(nw.ParentNetwork),
			strconv.FormatBool(isSuperNetwork),
			strconv.FormatBool(nat),
			strconv.FormatBool(underlay),
			clusterId,
		)

		if nw.Consumption == nil || nw.Consumption.Ipv4 == nil {
			continue
		}

		c.storeGauge(metalNetworkUsedIPs, float64(nw.Consumption.Ipv4.UsedIps), nwID)
		c.storeGauge(metalNetworkAvailableIps, float64(nw.Consumption.Ipv4.AvailableIps), nwID)
		c.storeGauge(metalNetworkUsedPrefixes, float64(nw.Consumption.Ipv4.UsedPrefixes), nwID)
		c.storeGauge(metalNetworkAvailablePrefixes, float64(nw.Consumption.Ipv4.AvailablePrefixes), nwID)
	}

	return nil
}

func (c *collector) partitionMetrics(ctx context.Context) error {
	resp, err := c.client.Adminv2().Partition().Capacity(ctx, &adminv2.PartitionServiceCapacityRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving partitions: %w", err)
	}

	for _, p := range resp.PartitionCapacity {
		for _, s := range p.MachineSizeCapacities {
			var (
				pID  = p.Partition
				size = s.Size
			)

			c.storeGauge(metalPartitionCapacityTotal, float64(s.Total), pID, size)
			c.storeGauge(metalPartitionCapacityAllocated, float64(s.Allocated), pID, size)
			c.storeGauge(metalPartitionCapacityWaiting, float64(s.Waiting), pID, size)
			c.storeGauge(metalPartitionCapacityFree, float64(s.Allocatable), pID, size)
			c.storeGauge(metalPartitionCapacityAllocatable, float64(s.Allocatable), pID, size)
			c.storeGauge(metalPartitionCapacityFaulty, float64(s.Faulty), pID, size)
			c.storeGauge(metalPartitionCapacityReservationsTotal, float64(s.Reservations), pID, size)
			c.storeGauge(metalPartitionCapacityReservationsUsed, float64(s.UsedReservations), pID, size)
			c.storeGauge(metalPartitionCapacityPhonedHome, float64(s.PhonedHome), pID, size)
			c.storeGauge(metalPartitionCapacityUnavailable, float64(s.Unavailable), pID, size)
			c.storeGauge(metalPartitionCapacityOther, float64(s.Other), pID, size)
		}
	}

	return nil
}

func (c *collector) imageMetrics(ctx context.Context) error {
	resp, err := c.client.Adminv2().Image().Usage(ctx, &adminv2.ImageServiceUsageRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving images: %w", err)
	}

	for _, i := range resp.ImageUsage {
		var imageFeatures []string
		for _, feature := range i.Image.Features {
			featureString, err := enum.GetStringValue(feature)
			if err != nil {
				continue
			}
			imageFeatures = append(imageFeatures, *featureString)
		}
		var (
			id             = i.Image.Id
			usage          = len(i.UsedBy)
			created        = fmt.Sprintf("%d", i.Image.Meta.CreatedAt.AsTime().Unix())
			expirationDate = fmt.Sprintf("%d", i.Image.ExpiresAt.AsTime().Unix())
			features       = strings.Join(imageFeatures, ",")
		)

		classification, err := enum.GetStringValue(i.Image.Classification)
		if err != nil {
			return fmt.Errorf("unable to get image classification string: %w", err)
		}

		c.storeGauge(metalImageUsedTotal, float64(usage), id, *i.Image.Name, *classification, created, expirationDate, features)
	}

	return nil
}

func (c *collector) projectMetrics(ctx context.Context) error {
	resp, err := c.client.Adminv2().Project().List(ctx, &adminv2.ProjectServiceListRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving images: %w", err)
	}

	for _, p := range resp.Projects {
		c.storeGauge(metalProjectInfo, float64(1.0), p.Uuid, p.Name, p.Tenant)
	}

	return nil
}

func (c *collector) switchMetrics(ctx context.Context) error {
	resp, err := c.client.Adminv2().Switch().List(ctx, &adminv2.SwitchServiceListRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving switches: %w", err)
	}

	for _, s := range resp.Switches {
		var (
			lastSync      = pointer.SafeDeref(s.LastSync).Time.AsTime()
			lastSyncError = pointer.SafeDeref(s.LastSyncError).Time.AsTime()

			syncFailed              = 0.0
			lastSyncDurationMs      = float64(pointer.SafeDeref(s.LastSync).Duration.AsDuration().Milliseconds())
			lastSyncErrorDurationMs = float64(pointer.SafeDeref(s.LastSyncError).Duration.AsDuration().Milliseconds())

			partitionID = s.Partition
			rackID      = pointer.SafeDeref(s.Rack)
			osVendor    = pointer.SafeDeref(s.Os).Vendor.String()
			osVersion   = pointer.SafeDeref(s.Os).Version
			// metal core version is very long: v0.9.1 (1d5e42ea), tags/v0.9.1-0-g1d5e42e, go1.20.5
			metalCoreVersion = strings.Split(pointer.SafeDeref(s.Os).MetalCoreVersion, ",")[0]
			metalCoreUp      = 1.0
			managementIP     = s.ManagementIp
		)

		if lastSyncError.After(lastSync) {
			syncFailed = 1.0
			lastSyncDurationMs = lastSyncErrorDurationMs
			lastSync = lastSyncError
		}

		if time.Since(lastSync) > 1*time.Minute {
			metalCoreUp = 0.0
		}

		c.storeGauge(metalSwitchInfo, 1.0, s.Id, partitionID, rackID, metalCoreVersion, osVendor, osVersion, managementIP)
		c.storeGauge(metalSwitchMetalCoreUp, metalCoreUp, s.Id, partitionID, rackID)
		c.storeGauge(metalSwitchSyncFailed, syncFailed, s.Id, partitionID, rackID)
		c.storeGaugeTimestamp(lastSync, metalSwitchSyncDurationsMs, lastSyncDurationMs, s.Id, partitionID, rackID)

		for _, conn := range s.MachineConnections {
			c.storeGauge(metalSwitchInterfaceInfo, 1.0, s.Id, pointer.SafeDeref(conn.Nic).Name, conn.MachineId, partitionID)
			if conn.Nic.BgpPortState != nil {
				c.storeGauge(switchInterfaceBGPTimeStampEstablished, float64(pointer.SafeDeref(conn.Nic.BgpPortState.BgpTimerUpEstablished).Seconds), s.Id, pointer.SafeDeref(conn.Nic).Name, conn.MachineId, partitionID)
			}

		}
	}

	return nil
}

func (c *collector) machineMetrics(ctx context.Context) error {
	machines, err := c.client.Adminv2().Machine().List(ctx, &adminv2.MachineServiceListRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving machines: %w", err)
	}

	machineBMCs, err := c.client.Adminv2().Machine().ListBMC(ctx, &adminv2.MachineServiceListBMCRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving machine bmcs: %w", err)
	}

	allIssues, err := c.client.Adminv2().Machine().Issues(ctx, &adminv2.MachineServiceIssuesRequest{})
	if err != nil {
		return fmt.Errorf("error retrieving machine issues list: %w", err)
	}

	issues, err := c.client.Adminv2().Machine().Issues(ctx, &adminv2.MachineServiceIssuesRequest{
		Query: &apiv2.MachineIssuesQuery{
			LastErrorThreshold: durationpb.New(time.Hour),
		},
	})
	if err != nil {
		return fmt.Errorf("error retrieving machine issues: %w", err)
	}

	issuesByMachineID := map[string][]string{}
	for _, issue := range issues.Issues {
		var issueTypes []string
		for _, i := range issue.Issues {
			typeString, err := enum.GetStringValue(i.Type)
			if err != nil {
				continue
			}
			issueTypes = append(issueTypes, *typeString)
		}
		issuesByMachineID[issue.Uuid] = issueTypes
	}

	allIssuesByID := map[string]bool{}
	for _, issue := range allIssues.Issues {
		allIssuesByID[issue.Uuid] = true
		for _, i := range issue.Issues {
			severityString, err := enum.GetStringValue(i.Severity)
			if err != nil {
				return err
			}
			c.storeGauge(metalMachineIssuesInfo, 1.0, issue.Uuid, i.Description, *severityString, i.ReferenceUrl)
		}
	}

	for _, m := range machines.Machines {
		var (
			partitionID = ""
			role        = ""
			hostname    = "NOTALLOCATED"
			clusterID   = "NOTALLOCATED"
			primaryASN  = "NOTALLOCATED"
			state       = "AVAILABLE"
			imageId     = "NOTALLOCATED"
		)

		if m.Status != nil && m.Status.Condition != nil {
			stateString, err := enum.GetStringValue(m.Status.Condition.State)
			if err != nil {
				return err
			}
			state = strings.ToUpper(*stateString)
		}

		if m.Allocation != nil {
			roleString, err := enum.GetStringValue(m.Allocation.AllocationType)
			if err != nil {
				return err
			}
			role = *roleString

			hostname = m.Allocation.Hostname

			if m.Allocation.Image != nil {
				imageId = m.Allocation.Image.Id
			}

			if m.Meta.Labels != nil && m.Meta.Labels.Labels != nil {
				if id, ok := m.Meta.Labels.Labels[tag.ClusterID]; ok {
					clusterID = id
				}
				if asn, ok := m.Meta.Labels.Labels[tag.MachineNetworkPrimaryASN]; ok {
					primaryASN = asn
				}
			}
		}
		if m.Partition != nil {
			partitionID = m.Partition.Id
		}

		if machineBMC, ok := machineBMCs.BmcReports[m.Uuid]; ok {
			var powerstate float64
			if machineBMC.Bmc != nil {
				switch machineBMC.Bmc.PowerState {
				case "ON":
					powerstate = 1
				case "OFF":
					powerstate = 0
				default:
					powerstate = -1
				}
				c.storeGauge(metalMachinePowerState, powerstate, m.Uuid)
			}

			c.storeGauge(metalMachinePowerSuppliesTotal, float64(len(machineBMC.PowerSupplies)), m.Uuid)

			healthy := 0
			for _, ps := range machineBMC.PowerSupplies {
				if ps.Health == "OK" {
					healthy++
				}
			}

			c.storeGauge(metalMachinePowerSuppliesHealthy, float64(healthy), m.Uuid)

			if machineBMC.PowerMetric != nil {
				c.storeGauge(metalMachinePowerUsage, float64(machineBMC.PowerMetric.AverageConsumedWatts), m.Uuid)
			}

			size := "UNKNOWN"
			if m.Size != nil {
				size = m.Size.Id
			}

			if machineBMC.Fru != nil {
				c.storeGauge(metalMachineHardwareInfo, 1.0, m.Uuid, partitionID, size,
					pointer.SafeDeref(machineBMC.Bmc).Version,
					pointer.SafeDeref(machineBMC.Bios).Version,
					pointer.SafeDeref(machineBMC.Fru.ChassisPartNumber),
					pointer.SafeDeref(machineBMC.Fru.ChassisPartSerial),
					pointer.SafeDeref(machineBMC.Fru.BoardMfg),
					pointer.SafeDeref(machineBMC.Fru.BoardMfgSerial),
					pointer.SafeDeref(machineBMC.Fru.BoardPartNumber),
					pointer.SafeDeref(machineBMC.Fru.ProductManufacturer),
					pointer.SafeDeref(machineBMC.Fru.ProductPartNumber),
					pointer.SafeDeref(machineBMC.Fru.ProductSerial),
				)
			}
		}

		c.storeGauge(metalMachineAllocationInfo, 1.0, m.Uuid, partitionID, hostname, clusterID, primaryASN, role, state, imageId)

		for issueID := range allIssuesByID {
			issues, ok := issuesByMachineID[m.Uuid]
			if !ok {
				c.storeGauge(metalMachineIssues, 0.0, m.Uuid, issueID)
				continue
			}

			if slices.Contains(issues, issueID) {
				c.storeGauge(metalMachineIssues, 1.0, m.Uuid, issueID)
			} else {
				c.storeGauge(metalMachineIssues, 0.0, m.Uuid, issueID)
			}
		}
	}

	return nil
}
