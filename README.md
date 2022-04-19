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

# HELP metal_partition_partitionCapacity_allocated The partitionCapacity of allocated machines in the partition
# TYPE metal_partition_partitionCapacity_allocated gauge
metal_partition_partitionCapacity_allocated{partition="fel-wps101",size="c1-xlarge-x86"} 148

# HELP metal_partition_partitionCapacity_faulty The partitionCapacity of faulty machines in the partition
# TYPE metal_partition_partitionCapacity_faulty gauge
metal_partition_partitionCapacity_faulty{partition="fel-wps101",size="c1-xlarge-x86"} 2

# HELP metal_partition_partitionCapacity_free The partitionCapacity of free machines in the partition
# TYPE metal_partition_partitionCapacity_free gauge
metal_partition_partitionCapacity_free{partition="fel-wps101",size="c1-xlarge-x86"} 5

# HELP metal_partition_partitionCapacity_total The total partitionCapacity of machines in the partition
# TYPE metal_partition_partitionCapacity_total gauge
metal_partition_partitionCapacity_total{partition="fel-wps101",size="c1-xlarge-x86"} 159
```
