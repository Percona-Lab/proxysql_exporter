package collector

import (
	_ "github.com/go-sql-driver/mysql" // register SQL driver
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "proxysql"

type Interface interface {
	Collect(ch chan<- prometheus.Metric)
}

// metric contains information about Prometheus metric.
type metric struct {
	name      string
	valueType prometheus.ValueType
	help      string
}
