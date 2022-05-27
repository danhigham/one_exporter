package main

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/OpenNebula/one/src/oca/go/src/goca"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	clusterMetrics = make(map[string]*prometheus.GaugeVec)
	hostMetrics    = make(map[string]*prometheus.GaugeVec)
)

type MonitoringData struct {
	XMLName    xml.Name `xml:"MONITORING_DATA"`
	Text       string   `xml:",chardata"`
	Monitoring []struct {
		Text      string `xml:",chardata"`
		Timestamp string `xml:"TIMESTAMP"`
		ID        string `xml:"ID"`
		Capacity  struct {
			Text    string `xml:",chardata"`
			FreeCPU string `xml:"FREE_CPU"`
			FreeMem string `xml:"FREE_MEMORY"`
			UsedCPU string `xml:"USED_CPU"`
			UsedMem string `xml:"USED_MEMORY"`
		} `xml:"CAPACITY"`
		SYSTEM string `xml:"SYSTEM"`
	} `xml:"MONITORING"`
}

func initMetrics() {
	clusterMetrics["TotalMem"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_totalmem",
		Help: "total memory available in cluster",
	}, []string{"cluster"})

	clusterMetrics["UsedMem"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_usedmem",
		Help: "real used memory in cluster",
	}, []string{"cluster"})

	clusterMetrics["MemUsage"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_memusage",
		Help: "total allocated memory in cluster",
	}, []string{"cluster"})

	clusterMetrics["TotalCPU"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_totalcpu",
		Help: "total cpu available in cluster",
	}, []string{"cluster"})

	clusterMetrics["UsedCPU"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_usedcpu",
		Help: "real used cpu in cluster",
	}, []string{"cluster"})

	clusterMetrics["CPUUsage"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_cpuusage",
		Help: "total allocated cpu in cluster",
	}, []string{"cluster"})

	clusterMetrics["RunningVMs"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_runningvms",
		Help: "running virtual machines in cluster",
	}, []string{"cluster"})

	clusterMetrics["ActiveHosts"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_cluster_activehosts",
		Help: "succesfully monitored hosts in cluster",
	}, []string{"cluster"})

	hostMetrics["TotalMem"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_totalmem",
		Help: "total memory available on host",
	}, []string{"cluster", "host"})

	hostMetrics["UsedMem"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_usedmem",
		Help: "real used memory on host",
	}, []string{"cluster", "host"})

	hostMetrics["MemUsage"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_memusage",
		Help: "total allocated memory on host",
	}, []string{"cluster", "host"})

	hostMetrics["TotalCPU"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_totalcpu",
		Help: "total cpu available on host",
	}, []string{"cluster", "host"})

	hostMetrics["UsedCPU"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_usedcpu",
		Help: "real used cpu on host",
	}, []string{"cluster", "host"})

	hostMetrics["CPUUsage"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_cpuusage",
		Help: "total allocated cpu on host",
	}, []string{"cluster", "host"})

	hostMetrics["RunningVMs"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_runningvms",
		Help: "running virtual machines on host",
	}, []string{"cluster", "host"})

	hostMetrics["DataStoreUsedMB"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_datastore_used_mb",
		Help: "host disk used in MB",
	}, []string{"cluster", "host"})

	hostMetrics["DataStoreFreeMB"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_datastore_free_mb",
		Help: "host disk free in MB",
	}, []string{"cluster", "host"})

	hostMetrics["DataStoreTotalMB"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "one_host_datastore_total_mb",
		Help: "host disk total in MB",
	}, []string{"cluster", "host"})

}

// recordMetrics from OpenNebula
func recordMetrics(config config, logger log.Logger) {

	level.Info(logger).Log("msg", "recording metrics from opennebula frontend", "interval", config.interval)

	client := goca.NewDefaultClient(goca.NewConfig(config.user, config.password, config.endpoint))
	controller := goca.NewController(client)

	for {

		pool, err := controller.Hosts().Info()
		if err != nil {
			level.Error(logger).Log("msg", "error retrieving hosts info", "error", err)
			return
		}

		type metrics struct {
			cluster, metric string
		}
		sum := make(map[metrics]int)

		for _, host := range pool.Hosts {
			monitoringRaw, _ := controller.Host(host.ID).Monitoring()

			// monitoring data from monitoring endpoint
			var monitoringData MonitoringData
			xml.Unmarshal([]byte(monitoringRaw), &monitoringData)

			// order monitoring data by timestamp
			sort.Slice(monitoringData.Monitoring, func(i, j int) bool {
				return monitoringData.Monitoring[i].Timestamp < monitoringData.Monitoring[j].Timestamp
			})

			lastValue := monitoringData.Monitoring[len(monitoringData.Monitoring)-1]

			host.Share.UsedMem, _ = strconv.Atoi(lastValue.Capacity.UsedMem)
			host.Share.UsedCPU, _ = strconv.Atoi(lastValue.Capacity.UsedCPU)

			level.Debug(logger).Log(fmt.Sprintf("%+v", host.Share.Datastores))

			level.Debug(logger).Log("msg", "host metrics",
				"host", host.Name,
				"TotalMem", host.Share.TotalMem,
				"UsedMem", host.Share.UsedMem,
				"TotalCPU", host.Share.TotalCPU,
				"UsedCPU", host.Share.UsedCPU,
				"RunningVMs", host.Share.RunningVMs)

			// record host metrics
			hostMetrics["TotalMem"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.TotalMem))
			hostMetrics["UsedMem"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.UsedMem))
			hostMetrics["MemUsage"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.MemUsage))
			hostMetrics["TotalCPU"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.TotalCPU))
			hostMetrics["UsedCPU"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.UsedCPU))
			hostMetrics["CPUUsage"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.CPUUsage))
			hostMetrics["RunningVMs"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.RunningVMs))
			hostMetrics["DataStoreUsedMB"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.Datastores[0].UsedMB))
			hostMetrics["DataStoreFreeMB"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.Datastores[0].FreeMB))
			hostMetrics["DataStoreTotalMB"].With(prometheus.Labels{"cluster": host.Cluster, "host": host.Name}).Set(float64(host.Share.Datastores[0].TotalMB))

			// sum cluster metrics
			sum[metrics{host.Cluster, "TotalMem"}] = sum[metrics{host.Cluster, "TotalMem"}] + host.Share.TotalMem
			sum[metrics{host.Cluster, "UsedMem"}] = sum[metrics{host.Cluster, "UsedMem"}] + host.Share.UsedMem
			sum[metrics{host.Cluster, "MemUsage"}] = sum[metrics{host.Cluster, "MemUsage"}] + host.Share.MemUsage
			sum[metrics{host.Cluster, "TotalCPU"}] = sum[metrics{host.Cluster, "TotalCPU"}] + host.Share.TotalCPU
			sum[metrics{host.Cluster, "UsedCPU"}] = sum[metrics{host.Cluster, "UsedCPU"}] + host.Share.UsedCPU
			sum[metrics{host.Cluster, "CPUUsage"}] = sum[metrics{host.Cluster, "CPUUsage"}] + host.Share.CPUUsage
			sum[metrics{host.Cluster, "RunningVMs"}] = sum[metrics{host.Cluster, "RunningVMs"}] + host.Share.RunningVMs

			if host.StateRaw == 2 {
				sum[metrics{host.Cluster, "ActiveHosts"}] = sum[metrics{host.Cluster, "ActiveHosts"}] + 1
			}
		}

		for key, value := range sum {
			// record cluster metrics
			clusterMetrics[key.metric].With(prometheus.Labels{"cluster": key.cluster}).Set(float64(value))
		}

		time.Sleep(time.Duration(config.interval) * time.Second)
	}
}
