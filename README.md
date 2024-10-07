# metal-metrics-exporter

A simple exporter for metal-api metrics.

## sample output

```text
# HELP metal_image_used_total The total number of machines using a image
# TYPE metal_image_used_total gauge
metal_image_used_total{classification="preview",created="1588078965",expirationDate="1598533365",features="firewall",imageID="firewall-ubuntu-2.0.20200331",name="Firewall 2 Ubuntu 20200331"} 2

# HELP metal_network_info Shows available prefixes in a network
# TYPE metal_network_info gauge
metal_network_info{description="",destPrefixes="",isPrivateSuper="false",isUnderlay="false",name="ausfalltest",networkId="020966ab-18da-40d6-ba69-fea8d60e6074",parentNetworkID="tenant-super-network-nbg-w8101",partition="nbg-w8101",prefixes="10.91.112.0/22",projectId="c77daafe-58f8-44df-82eb-6ef631cee3c9",useNat="false",vrf="454"} 1

# HELP metal_network_ip_available The total number of available IPs of the network
# TYPE metal_network_ip_available gauge
metal_network_ip_available{networkId="020966ab-18da-40d6-ba69-fea8d60e6074"} 1024

# HELP metal_network_ip_used The total number of used IPs of the network
# TYPE metal_network_ip_used gauge
metal_network_ip_used{networkId="020966ab-18da-40d6-ba69-fea8d60e6074"} 2

# HELP metal_network_prefix_available The total number of available prefixes of the network
# TYPE metal_network_prefix_available gauge
metal_network_prefix_available{networkId="020966ab-18da-40d6-ba69-fea8d60e6074"} 256

# HELP metal_network_prefix_used The total number of used prefixes of the network
# TYPE metal_network_prefix_used gauge
metal_network_prefix_used{networkId="020966ab-18da-40d6-ba69-fea8d60e6074"} 0

# HELP metal_partition_capacity_allocatable The total number of waiting allocatable machines in the partition
# TYPE metal_partition_capacity_allocatable gauge
metal_partition_capacity_allocatable{partition="fra-equ01",size="c1-xlarge-x86"} 2

# HELP metal_partition_capacity_allocated The capacity of allocated machines in the partition
# TYPE metal_partition_capacity_allocated gauge
metal_partition_capacity_allocated{partition="fra-equ01",size="c1-xlarge-x86"} 1

# HELP metal_partition_capacity_faulty The capacity of faulty machines in the partition
# TYPE metal_partition_capacity_faulty gauge
metal_partition_capacity_faulty{partition="fra-equ01",size="c1-xlarge-x86"} 0

# HELP metal_partition_capacity_free (DEPRECATED) The total number of allocatable machines in the partition, use metal_partition_capacity_allocatable
# TYPE metal_partition_capacity_free gauge
metal_partition_capacity_free{partition="fra-equ01",size="c1-xlarge-x86"} 2

# HELP metal_partition_capacity_other The total number of machines in an other state in the partition
# TYPE metal_partition_capacity_other gauge
metal_partition_capacity_other{partition="fra-equ01",size="c1-xlarge-x86"} 0

# HELP metal_partition_capacity_phoned_home The total number of faulty machines in the partition
# TYPE metal_partition_capacity_phoned_home gauge
metal_partition_capacity_phoned_home{partition="fra-equ01",size="c1-xlarge-x86"} 1

# HELP metal_partition_capacity_reservations_total The sum of capacity reservations in the partition
# TYPE metal_partition_capacity_reservations_total gauge
metal_partition_capacity_reservations_total{partition="fra-equ01",size="c1-xlarge-x86"} 1

# HELP metal_partition_capacity_reservations_used The sum of used capacity reservations in the partition
# TYPE metal_partition_capacity_reservations_used gauge
metal_partition_capacity_reservations_used{partition="fra-equ01",size="c1-xlarge-x86"} 0

# HELP metal_partition_capacity_total The total number of machines in the partition
# TYPE metal_partition_capacity_total gauge
metal_partition_capacity_total{partition="fra-equ01",size="c1-xlarge-x86"} 3

# HELP metal_partition_capacity_unavailable The total number of faulty machines in the partition
# TYPE metal_partition_capacity_unavailable gauge
metal_partition_capacity_unavailable{partition="fra-equ01",size="c1-xlarge-x86"} 0

# HELP metal_partition_capacity_waiting The total number of waiting machines in the partition
# TYPE metal_partition_capacity_waiting gauge
metal_partition_capacity_waiting{partition="fra-equ01",size="c1-xlarge-x86"} 2

# HELP metal_switch_sync_durations The duration of the syncs
# TYPE metal_switch_sync_durations gauge
metal_switch_sync_durations{partition="fra-equ01",rackid="fra-equ01-rack01",switchname="fra-equ01-r01leaf01"} 2.06530044e+08
metal_switch_sync_durations{partition="fra-equ01",rackid="fra-equ01-rack01",switchname="fra-equ01-r01leaf02"} 2.24029886e+08

# HELP metal_switch_sync_failed 1 when the switch sync is failing, otherwise 0
# TYPE metal_switch_sync_failed gauge
metal_switch_sync_failed{partition="fra-equ01",rackid="fra-equ01-rack01",switchname="fra-equ01-r01leaf01"} 0
metal_switch_sync_failed{partition="fra-equ01",rackid="fra-equ01-rack01",switchname="fra-equ01-r01leaf02"} 0

# HELP metal_machine_hardware_info Provide information about the machine
# TYPE metal_machine_hardware_info gauge
metal_machine_hardware_info{biosVersion="3.4",bmcVersion="1.73",boardMfg="Supermicro",boardMfgSerial="",boardPartNumber="X11DPT-B",chassisPartNumber="CSE-217BHQ+-R2K22BP2",chassisPartSerial="C217BAH24AE0006",machineid="11111111-2222-3333-4444-aabbccddeeff",partition="fra-equ01",productManufacturer="Supermicro",productPartNumber="SYS-2029BT-HNR",productSerial="E262335X9100766D"} 1

# HELP metal_machine_ipmi_info Provide the ipmi ip address
# TYPE metal_machine_ipmi_info gauge
metal_machine_ipmi_info{ipmiIP="10.1.1.36:623",machineid="11111111-2222-3333-4444-aabbccddeeff"} 1

# HELP metal_machine_ipmi_power_state Provide information about the machine power state
# TYPE metal_machine_ipmi_power_state gauge
metal_machine_ipmi_power_state{machineid="11111111-2222-3333-4444-aabbccddeeff"} 1

# HELP metal_machine_ipmi_power_supplies_healthy Provide information about the number of healthy power supplies
# TYPE metal_machine_ipmi_power_supplies_healthy gauge
metal_machine_ipmi_power_supplies_healthy{machineid="11111111-2222-3333-4444-aabbccddeeff"} 2

# HELP metal_machine_ipmi_power_supplies_total Provide information about the total number of power supplies
# TYPE metal_machine_ipmi_power_supplies_total gauge
metal_machine_ipmi_power_supplies_total{machineid="11111111-2222-3333-4444-aabbccddeeff"} 2

# HELP metal_machine_ipmi_power_usage Provide information about the machine power usage in watts
# TYPE metal_machine_ipmi_power_usage gauge
metal_machine_ipmi_power_usage{machineid="11111111-2222-3333-4444-aabbccddeeff"} 69
```
