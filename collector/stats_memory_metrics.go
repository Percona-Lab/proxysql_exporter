package collector

import (
	"database/sql"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// statsMemoryMetricsCollector extracts all metrics from stats_memory_metrics.
type statsMemoryMetricsCollector struct {
	db *sql.DB
	l  log.Logger
}

func NewStatsMemoryMetricsCollector(db *sql.DB) Interface {
	return &statsMemoryMetricsCollector{
		db: db,
		l:  log.NewLogger(os.Stderr),
	}
}

var statsMemoryMetrics = map[string]*metric{
	"jemalloc_allocated": {
		name:      "jemalloc_allocated",
		valueType: prometheus.GaugeValue,
		help:      "Bytes allocated by the application.",
	},
	"jemalloc_active": {
		name:      "jemalloc_active",
		valueType: prometheus.GaugeValue,
		help:      "Bytes in pages allocated by the application.",
	},
	"jemalloc_mapped": {
		name:      "jemalloc_mapped",
		valueType: prometheus.GaugeValue,
		help:      "Bytes in extents mapped by the allocator.",
	},
	"jemalloc_metadata": {
		name:      "jemalloc_metadata",
		valueType: prometheus.GaugeValue,
		help:      "Bytes dedicated to metadata.",
	},
	"jemalloc_resident": {
		name:      "jemalloc_resident",
		valueType: prometheus.GaugeValue,
		help:      "Bytes in physically resident data pages mapped by the allocator.",
	},
	"auth_memory": {
		name:      "auth_memory",
		valueType: prometheus.GaugeValue,
		help:      "Memory used by the authentication module to store user credentials and attributes.",
	},
	"sqlite3_memory_bytes": {
		name:      "sqlite3_memory_bytes",
		valueType: prometheus.GaugeValue,
		help:      "Memory used by the embedded SQLite.",
	},
	"query_digest_memory": {
		name:      "query_digest_memory",
		valueType: prometheus.GaugeValue,
		help:      "Memory used to store data related to stats_mysql_query_digest.",
	},
}

func (c *statsMemoryMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	rows, err := c.db.Query(`SELECT Variable_Name, Variable_Value FROM stats_memory_metrics`)
	if err != nil {
		c.l.Error(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var value float64
		if err = rows.Scan(&name, &value); err != nil {
			c.l.Warn(err)
			continue
		}
		name = strings.ToLower(name)

		m := statsMemoryMetrics[name]
		if m == nil {
			m = &metric{
				name:      name,
				valueType: prometheus.UntypedValue,
				help:      "Undocumented stats_memory_metrics metric.",
			}
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "stats_memory", m.name),
				m.help,
				nil, nil,
			),
			m.valueType,
			value,
		)
	}

	if err = rows.Err(); err != nil {
		c.l.Warn(err)
	}
}

// check interfaces
var (
	_ Interface = (*statsMemoryMetricsCollector)(nil)
)
