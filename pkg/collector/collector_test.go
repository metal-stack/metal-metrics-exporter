package collector

import (
	"context"
	"log/slog"
	"maps"
	"math"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	apiv2client "github.com/metal-stack/api/go/client"
	adminv2 "github.com/metal-stack/api/go/metalstack/admin/v2"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/api/go/tag"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testCollector(t *testing.T, calls ...apiv2client.ClientCall) *collector {
	t.Helper()

	t.Setenv(apiv2client.TokenEnvName, "")
	t.Setenv(apiv2client.TokenFileEnvName, "")

	cl, err := apiv2client.New(&apiv2client.DialConfig{
		BaseURL:      "http://this-is-just-for-testing",
		Interceptors: []connect.Interceptor{apiv2client.NewTestInterceptor(t, calls)},
		Log:          slog.Default(),
		UserAgent:    "metal-metrics-exporter-test",
	})
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}

	return New(cl, time.Minute)
}

func runMetrics(t *testing.T, c *collector, fn func(context.Context) error) {
	t.Helper()

	if err := fn(t.Context()); err != nil {
		t.Fatalf("collecting metrics: %v", err)
	}

	c.currentMetrics = c.newMetrics
}

func compare(t *testing.T, c *collector, expected string, names ...string) {
	t.Helper()

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), names...); err != nil {
		t.Fatalf("unexpected metrics:\n%v", err)
	}
}

type sample struct {
	labels map[string]string
	value  float64
}

func gatherMetrics(t *testing.T, c *collector) map[string][]sample {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatalf("registering collector: %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	out := map[string][]sample{}
	for _, mf := range families {
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}

			var value float64
			switch {
			case m.GetGauge() != nil:
				value = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				value = m.GetCounter().GetValue()
			}

			out[mf.GetName()] = append(out[mf.GetName()], sample{labels: labels, value: value})
		}
	}

	return out
}

func assertMetricApprox(t *testing.T, c *collector, name string, labels map[string]string, want, tolerance float64) {
	t.Helper()

	for _, s := range gatherMetrics(t, c)[name] {
		if !maps.Equal(s.labels, labels) {
			continue
		}
		if math.Abs(s.value-want) > tolerance {
			t.Fatalf("%s%v = %v, want %v ± %v", name, labels, s.value, want, tolerance)
		}
		return
	}

	t.Fatalf("metric %s with labels %v not found", name, labels)
}

func TestNetworkMetrics(t *testing.T) {
	network := &apiv2.Network{
		Id:                  "network-1",
		Name:                new("network name"),
		Project:             new("project-1"),
		Description:         new("description"),
		Partition:           new("partition-1"),
		Vrf:                 new(uint32(42)),
		Prefixes:            []string{"10.0.0.0/24", "10.0.1.0/24"},
		DestinationPrefixes: []string{"0.0.0.0/0"},
		ParentNetwork:       new("parent-1"),
		Type:                apiv2.NetworkType_NETWORK_TYPE_SUPER,
		NatType:             apiv2.NATType_NAT_TYPE_IPV4_MASQUERADE,
		Meta:                &apiv2.Meta{Labels: &apiv2.Labels{Labels: map[string]string{tag.ClusterID: "cluster-1"}}},
		Consumption: &apiv2.NetworkConsumption{
			Ipv4: &apiv2.NetworkUsage{
				AvailableIps:      100,
				UsedIps:           10,
				AvailablePrefixes: 5,
				UsedPrefixes:      1,
			},
		},
	}

	c := testCollector(t, apiv2client.ClientCall{
		WantRequest: &adminv2.NetworkServiceListRequest{},
		WantResponse: func() connect.AnyResponse {
			return connect.NewResponse(&adminv2.NetworkServiceListResponse{Networks: []*apiv2.Network{network}})
		},
	})

	runMetrics(t, c, c.networkMetrics)

	compare(t, c, `
# HELP metal_network_info Provide information about the network
# TYPE metal_network_info gauge
metal_network_info{clusterTag="cluster-1",description="description",destPrefixes="0.0.0.0/0",isPrivateSuper="true",isUnderlay="false",name="network name",networkId="network-1",parentNetworkID="parent-1",partition="partition-1",prefixes="10.0.0.0/24,10.0.1.0/24",projectId="project-1",useNat="true",vrf="42"} 1
# HELP metal_network_ip_available The total number of available IPs of the network
# TYPE metal_network_ip_available gauge
metal_network_ip_available{networkId="network-1"} 100
# HELP metal_network_ip_used The total number of used IPs of the network
# TYPE metal_network_ip_used gauge
metal_network_ip_used{networkId="network-1"} 10
# HELP metal_network_prefix_available The total number of available prefixes of the network
# TYPE metal_network_prefix_available gauge
metal_network_prefix_available{networkId="network-1"} 5
# HELP metal_network_prefix_used The total number of used prefixes of the network
# TYPE metal_network_prefix_used gauge
metal_network_prefix_used{networkId="network-1"} 1
`,
		"metal_network_info",
		"metal_network_ip_available",
		"metal_network_ip_used",
		"metal_network_prefix_available",
		"metal_network_prefix_used",
	)
}

func TestPartitionMetrics(t *testing.T) {
	capacity := &adminv2.PartitionCapacity{
		Partition: "partition-1",
		MachineSizeCapacities: []*adminv2.MachineSizeCapacity{
			{
				Size:             "size-1",
				Total:            10,
				Allocated:        2,
				Waiting:          3,
				Allocatable:      4,
				Faulty:           1,
				Reservations:     5,
				UsedReservations: 2,
				PhonedHome:       1,
				Unavailable:      1,
				Other:            2,
			},
		},
	}

	c := testCollector(t, apiv2client.ClientCall{
		WantRequest: &adminv2.PartitionServiceCapacityRequest{},
		WantResponse: func() connect.AnyResponse {
			return connect.NewResponse(&adminv2.PartitionServiceCapacityResponse{PartitionCapacity: []*adminv2.PartitionCapacity{capacity}})
		},
	})

	runMetrics(t, c, c.partitionMetrics)

	compare(t, c, `
# HELP metal_partition_capacity_allocatable The total number of waiting allocatable machines in the partition
# TYPE metal_partition_capacity_allocatable gauge
metal_partition_capacity_allocatable{partition="partition-1",size="size-1"} 4
# HELP metal_partition_capacity_allocated The capacity of allocated machines in the partition
# TYPE metal_partition_capacity_allocated gauge
metal_partition_capacity_allocated{partition="partition-1",size="size-1"} 2
# HELP metal_partition_capacity_faulty The capacity of faulty machines in the partition
# TYPE metal_partition_capacity_faulty gauge
metal_partition_capacity_faulty{partition="partition-1",size="size-1"} 1
# HELP metal_partition_capacity_free (DEPRECATED) The total number of allocatable machines in the partition, use metal_partition_capacity_allocatable
# TYPE metal_partition_capacity_free gauge
metal_partition_capacity_free{partition="partition-1",size="size-1"} 4
# HELP metal_partition_capacity_other The total number of machines in an other state in the partition
# TYPE metal_partition_capacity_other gauge
metal_partition_capacity_other{partition="partition-1",size="size-1"} 2
# HELP metal_partition_capacity_phoned_home The total number of faulty machines in the partition
# TYPE metal_partition_capacity_phoned_home gauge
metal_partition_capacity_phoned_home{partition="partition-1",size="size-1"} 1
# HELP metal_partition_capacity_reservations_total The sum of capacity reservations in the partition
# TYPE metal_partition_capacity_reservations_total gauge
metal_partition_capacity_reservations_total{partition="partition-1",size="size-1"} 5
# HELP metal_partition_capacity_reservations_used The sum of used capacity reservations in the partition
# TYPE metal_partition_capacity_reservations_used gauge
metal_partition_capacity_reservations_used{partition="partition-1",size="size-1"} 2
# HELP metal_partition_capacity_total The total number of machines in the partition
# TYPE metal_partition_capacity_total gauge
metal_partition_capacity_total{partition="partition-1",size="size-1"} 10
# HELP metal_partition_capacity_unavailable The total number of unavailable machines in the partition
# TYPE metal_partition_capacity_unavailable gauge
metal_partition_capacity_unavailable{partition="partition-1",size="size-1"} 1
# HELP metal_partition_capacity_waiting The total number of waiting machines in the partition
# TYPE metal_partition_capacity_waiting gauge
metal_partition_capacity_waiting{partition="partition-1",size="size-1"} 3
`,
		"metal_partition_capacity_allocatable",
		"metal_partition_capacity_allocated",
		"metal_partition_capacity_faulty",
		"metal_partition_capacity_free",
		"metal_partition_capacity_other",
		"metal_partition_capacity_phoned_home",
		"metal_partition_capacity_reservations_total",
		"metal_partition_capacity_reservations_used",
		"metal_partition_capacity_total",
		"metal_partition_capacity_unavailable",
		"metal_partition_capacity_waiting",
	)
}

func TestImageMetrics(t *testing.T) {
	usage := &apiv2.ImageUsage{
		Image: &apiv2.Image{
			Id:             "image-1",
			Name:           new("image name"),
			Features:       []apiv2.ImageFeature{apiv2.ImageFeature_IMAGE_FEATURE_FIREWALL},
			Classification: apiv2.ImageClassification_IMAGE_CLASSIFICATION_PREVIEW,
			Meta:           &apiv2.Meta{CreatedAt: timestamppb.New(time.Unix(1000, 0))},
			ExpiresAt:      timestamppb.New(time.Unix(2000, 0)),
		},
		UsedBy: []string{"machine-1", "machine-2"},
	}

	c := testCollector(t, apiv2client.ClientCall{
		WantRequest: &adminv2.ImageServiceUsageRequest{},
		WantResponse: func() connect.AnyResponse {
			return connect.NewResponse(&adminv2.ImageServiceUsageResponse{ImageUsage: []*apiv2.ImageUsage{usage}})
		},
	})

	runMetrics(t, c, c.imageMetrics)

	compare(t, c, `
# HELP metal_image_used_total The total number of machines using a image
# TYPE metal_image_used_total gauge
metal_image_used_total{classification="preview",created="1000",expirationDate="2000",features="firewall",imageID="image-1",name="image name"} 2
`,
		"metal_image_used_total",
	)
}

func TestProjectMetrics(t *testing.T) {
	c := testCollector(t, apiv2client.ClientCall{
		WantRequest: &adminv2.ProjectServiceListRequest{},
		WantResponse: func() connect.AnyResponse {
			return connect.NewResponse(&adminv2.ProjectServiceListResponse{
				Projects: []*apiv2.Project{{Uuid: "project-1", Name: "project name", Tenant: "tenant-1"}},
			})
		},
	})

	runMetrics(t, c, c.projectMetrics)

	compare(t, c, `
# HELP metal_project_info Provide information about metal projects
# TYPE metal_project_info gauge
metal_project_info{name="project name",projectId="project-1",tenantId="tenant-1"} 1
`,
		"metal_project_info",
	)
}

func TestComponentMetrics(t *testing.T) {
	withToken := &apiv2.Component{
		Uuid:       "component-1",
		Type:       apiv2.ComponentType_COMPONENT_TYPE_METAL_METRICS_EXPORTER,
		Identifier: "identifier-1",
		StartedAt:  timestamppb.New(time.Unix(1000, 0)),
		ReportedAt: timestamppb.New(time.Unix(2000, 0)),
		Interval:   durationpb.New(time.Hour),
		Version: &apiv2.Version{
			Version:  "v1.0.0",
			Revision: "rev-1",
			GitSha1:  "abc123",
		},
		Token: &apiv2.Token{
			Uuid:    "token-1",
			User:    "user-1",
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
	}
	withoutToken := &apiv2.Component{
		Uuid:       "component-2",
		Type:       apiv2.ComponentType_COMPONENT_TYPE_METAL_METRICS_EXPORTER,
		Identifier: "identifier-2",
		StartedAt:  timestamppb.New(time.Unix(3000, 0)),
		ReportedAt: timestamppb.New(time.Unix(4000, 0)),
		Interval:   durationpb.New(2000000 * time.Hour),
		Version:    &apiv2.Version{Version: "v2.0.0"},
	}

	c := testCollector(t, apiv2client.ClientCall{
		WantRequest: &adminv2.ComponentServiceListRequest{},
		WantResponse: func() connect.AnyResponse {
			return connect.NewResponse(&adminv2.ComponentServiceListResponse{Components: []*apiv2.Component{withToken, withoutToken}})
		},
	})

	runMetrics(t, c, c.componentMetrics)

	compare(t, c, `
# HELP metal_component_info Provide information about components connected to the metal-apiserver
# TYPE metal_component_info gauge
metal_component_info{gitSHA1="",identifier="identifier-2",interval="2000000h0m0s",reportedAt="4000",revision="",startedAt="3000",type="metal-metrics-exporter",uuid="component-2",version="v2.0.0"} 1
metal_component_info{gitSHA1="abc123",identifier="identifier-1",interval="1h0m0s",reportedAt="2000",revision="rev-1",startedAt="1000",type="metal-metrics-exporter",uuid="component-1",version="v1.0.0"} 1
# HELP metal_component_up 1 when the component reported within its ping interval, otherwise 0
# TYPE metal_component_up gauge
metal_component_up{identifier="identifier-1",type="metal-metrics-exporter",uuid="component-1"} 0
metal_component_up{identifier="identifier-2",type="metal-metrics-exporter",uuid="component-2"} 1
`,
		"metal_component_info",
		"metal_component_up",
	)

	assertMetricApprox(t, c, "metal_component_token_lifetime_seconds", map[string]string{
		"uuid":       "component-1",
		"type":       "metal-metrics-exporter",
		"identifier": "identifier-1",
		"token":      "token-1",
		"user":       "user-1",
	}, time.Hour.Seconds(), 5)
}

func TestSwitchMetrics(t *testing.T) {
	sw := &apiv2.Switch{
		Id:           "switch-1",
		Partition:    "partition-1",
		Rack:         new("rack-1"),
		ManagementIp: "10.0.0.1",
		Os: &apiv2.SwitchOS{
			Vendor:           apiv2.SwitchOSVendor_SWITCH_OS_VENDOR_SONIC,
			Version:          "1.2.3",
			MetalCoreVersion: "v0.9.1 (abcdef), tags/v0.9.1-0-g1d5e42e, go1.20.5",
		},
		LastSync: &apiv2.SwitchSync{
			Time:     timestamppb.New(time.Unix(1000, 0)),
			Duration: durationpb.New(100 * time.Millisecond),
		},
		LastSyncError: &apiv2.SwitchSync{
			Time:     timestamppb.New(time.Unix(500, 0)),
			Duration: durationpb.New(200 * time.Millisecond),
		},
		MachineConnections: []*apiv2.MachineConnection{
			{
				MachineId: "machine-1",
				Nic: &apiv2.SwitchNic{
					Name:         "Ethernet1",
					BgpPortState: &apiv2.SwitchBGPPortState{BgpTimerUpEstablished: timestamppb.New(time.Unix(1000, 0))},
				},
			},
		},
	}

	c := testCollector(t, apiv2client.ClientCall{
		WantRequest: &adminv2.SwitchServiceListRequest{},
		WantResponse: func() connect.AnyResponse {
			return connect.NewResponse(&adminv2.SwitchServiceListResponse{Switches: []*apiv2.Switch{sw}})
		},
	})

	runMetrics(t, c, c.switchMetrics)

	compare(t, c, `
# HELP metal_switch_info Provide information about the switch
# TYPE metal_switch_info gauge
metal_switch_info{managementIP="10.0.0.1",metalCoreVersion="v0.9.1 (abcdef)",osVendor="SONiC",osVersion="1.2.3",partition="partition-1",rackid="rack-1",switchname="switch-1"} 1
# HELP metal_switch_interface_bgp_established_timestamp Provide the unix timestamp since BGP is established on the interfaces (0 if not established)
# TYPE metal_switch_interface_bgp_established_timestamp gauge
metal_switch_interface_bgp_established_timestamp{device="Ethernet1",machineid="machine-1",partition="partition-1",switchname="switch-1"} 1000
# HELP metal_switch_interface_info Provide information about the switch interfaces
# TYPE metal_switch_interface_info gauge
metal_switch_interface_info{device="Ethernet1",machineid="machine-1",partition="partition-1",switchname="switch-1"} 1
# HELP metal_switch_metal_core_up 1 when the metal-core is up, otherwise 0
# TYPE metal_switch_metal_core_up gauge
metal_switch_metal_core_up{partition="partition-1",rackid="rack-1",switchname="switch-1"} 0
# HELP metal_switch_sync_durations_ms The duration of the syncs in milliseconds
# TYPE metal_switch_sync_durations_ms gauge
metal_switch_sync_durations_ms{partition="partition-1",rackid="rack-1",switchname="switch-1"} 100 1000000
# HELP metal_switch_sync_failed 1 when the switch sync is failing, otherwise 0
# TYPE metal_switch_sync_failed gauge
metal_switch_sync_failed{partition="partition-1",rackid="rack-1",switchname="switch-1"} 0
`,
		"metal_switch_info",
		"metal_switch_interface_bgp_established_timestamp",
		"metal_switch_interface_info",
		"metal_switch_metal_core_up",
		"metal_switch_sync_durations_ms",
		"metal_switch_sync_failed",
	)
}

func TestMachineMetrics(t *testing.T) {
	machine := &apiv2.Machine{
		Uuid: "machine-1",
		Meta: &apiv2.Meta{Labels: &apiv2.Labels{Labels: map[string]string{
			tag.ClusterID:                "cluster-1",
			tag.MachineNetworkPrimaryASN: "asn-1",
		}}},
		Partition: &apiv2.Partition{Id: "partition-1"},
		Size:      &apiv2.Size{Id: "size-1"},
		Status: &apiv2.MachineStatus{
			Condition: &apiv2.MachineCondition{State: apiv2.MachineState_MACHINE_STATE_AVAILABLE},
		},
		Allocation: &apiv2.MachineAllocation{
			AllocationType: apiv2.MachineAllocationType_MACHINE_ALLOCATION_TYPE_MACHINE,
			Hostname:       "host-1",
			Image:          &apiv2.Image{Id: "image-1"},
		},
	}

	bmcDetails := &apiv2.MachineBMCDetails{
		Uuid: "machine-1",
		BmcReport: &apiv2.MachineBMCReport{
			Bmc:  &apiv2.MachineBMC{PowerState: "ON", Version: "bmc-1.0"},
			Bios: &apiv2.MachineBios{Version: "bios-1.0"},
			Fru: &apiv2.MachineFRU{
				ChassisPartNumber:   new("chassis-pn"),
				ChassisPartSerial:   new("chassis-sn"),
				BoardMfg:            new("board-mfg"),
				BoardMfgSerial:      new("board-mfg-sn"),
				BoardPartNumber:     new("board-pn"),
				ProductManufacturer: new("product-manufacturer"),
				ProductPartNumber:   new("product-pn"),
				ProductSerial:       new("product-sn"),
			},
			PowerMetric: &apiv2.MachinePowerMetric{AverageConsumedWatts: 100},
			PowerSupplies: []*apiv2.MachinePowerSupply{
				{Health: "OK"},
				{Health: "OK"},
				{Health: "Critical"},
			},
		},
	}

	issues := &adminv2.MachineServiceIssuesResponse{
		Issues: []*apiv2.MachineIssues{
			{
				Uuid: "machine-1",
				Issues: []*apiv2.MachineIssue{
					{
						Type:         apiv2.MachineIssueType_MACHINE_ISSUE_TYPE_ASN_UNIQUENESS,
						Severity:     apiv2.MachineIssueSeverity_MACHINE_ISSUE_SEVERITY_MAJOR,
						Description:  "asn is not unique",
						ReferenceUrl: "https://example.com/asn",
					},
				},
			},
		},
	}

	c := testCollector(t,
		apiv2client.ClientCall{
			WantRequest: &adminv2.MachineServiceListRequest{},
			WantResponse: func() connect.AnyResponse {
				return connect.NewResponse(&adminv2.MachineServiceListResponse{Machines: []*apiv2.Machine{machine}})
			},
		},
		apiv2client.ClientCall{
			WantRequest: &adminv2.MachineServiceListBMCRequest{},
			WantResponse: func() connect.AnyResponse {
				return connect.NewResponse(&adminv2.MachineServiceListBMCResponse{BmcDetails: []*apiv2.MachineBMCDetails{bmcDetails}})
			},
		},
		apiv2client.ClientCall{
			WantRequest: &adminv2.MachineServiceIssuesRequest{},
			WantResponse: func() connect.AnyResponse {
				return connect.NewResponse(issues)
			},
		},
		apiv2client.ClientCall{
			WantRequest: &adminv2.MachineServiceIssuesRequest{
				Query: &apiv2.MachineIssuesQuery{LastErrorThreshold: durationpb.New(time.Hour)},
			},
			WantResponse: func() connect.AnyResponse {
				return connect.NewResponse(issues)
			},
		},
	)

	runMetrics(t, c, c.machineMetrics)

	compare(t, c, `
# HELP metal_machine_allocation_info Provide information about the machine allocation
# TYPE metal_machine_allocation_info gauge
metal_machine_allocation_info{clusterTag="cluster-1",imageId="image-1",machineid="machine-1",machinename="host-1",partition="partition-1",primaryASN="asn-1",role="machine",state="AVAILABLE"} 1
# HELP metal_machine_hardware_info Provide information about the machine
# TYPE metal_machine_hardware_info gauge
metal_machine_hardware_info{biosVersion="bios-1.0",bmcVersion="bmc-1.0",boardMfg="board-mfg",boardMfgSerial="board-mfg-sn",boardPartNumber="board-pn",chassisPartNumber="chassis-pn",chassisPartSerial="chassis-sn",machineid="machine-1",partition="partition-1",productManufacturer="product-manufacturer",productPartNumber="product-pn",productSerial="product-sn",size="size-1"} 1
# HELP metal_machine_issues Provide information on machine issues
# TYPE metal_machine_issues gauge
metal_machine_issues{issueid="asn-not-unique",machineid="machine-1"} 1
# HELP metal_machine_issues_info Provide general information on issues that are evaluated by the metal-api
# TYPE metal_machine_issues_info gauge
metal_machine_issues_info{description="asn is not unique",issueid="asn-not-unique",refurl="https://example.com/asn",severity="major"} 1
# HELP metal_machine_power_state Provide information about the machine power state
# TYPE metal_machine_power_state gauge
metal_machine_power_state{machineid="machine-1"} 1
# HELP metal_machine_power_supplies_healthy Provide information about the number of healthy power supplies
# TYPE metal_machine_power_supplies_healthy gauge
metal_machine_power_supplies_healthy{machineid="machine-1"} 2
# HELP metal_machine_power_supplies_total Provide information about the total number of power supplies
# TYPE metal_machine_power_supplies_total gauge
metal_machine_power_supplies_total{machineid="machine-1"} 3
# HELP metal_machine_power_usage Provide information about the machine power usage in watts
# TYPE metal_machine_power_usage gauge
metal_machine_power_usage{machineid="machine-1"} 100
`,
		"metal_machine_allocation_info",
		"metal_machine_hardware_info",
		"metal_machine_issues",
		"metal_machine_issues_info",
		"metal_machine_power_state",
		"metal_machine_power_supplies_healthy",
		"metal_machine_power_supplies_total",
		"metal_machine_power_usage",
	)
}
