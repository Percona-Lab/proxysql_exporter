// Copyright 2016-2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const namespace = "proxysql"

// Exporter collects ProxySQL metrics.
// It implements prometheus.Collector interface.
type Exporter struct {
	dsn                            string
	scrapeMySQLGlobal              bool
	scrapeMySQLConnectionPool      bool
	scrapeMySQLConnectionList      bool
	scrapeDetailedMySQLProcessList bool
	scrapeMemoryMetrics            bool
	scrapesTotal                   prometheus.Counter
	scrapeErrorsTotal              *prometheus.CounterVec
	lastScrapeError                prometheus.Gauge
	lastScrapeDurationSeconds      prometheus.Gauge
	proxysqlUp                     prometheus.Gauge
}

// NewExporter returns a new ProxySQL exporter for the provided DSN.
// It scrapes stats_mysql_global, stats_mysql_connection_pool and stats_mysql_processlist if corresponding parameters are true.
func NewExporter(
	dsn string,
	scrapeMySQLGlobal bool,
	scrapeMySQLConnectionPool bool,
	scrapeMySQLConnectionList bool,
	scrapeDetailedMySQLProcessList bool,
	scrapeMemoryMetrics bool,
) *Exporter {
	return &Exporter{
		dsn:                            dsn,
		scrapeMySQLGlobal:              scrapeMySQLGlobal,
		scrapeMySQLConnectionPool:      scrapeMySQLConnectionPool,
		scrapeMySQLConnectionList:      scrapeMySQLConnectionList,
		scrapeDetailedMySQLProcessList: scrapeDetailedMySQLProcessList,
		scrapeMemoryMetrics:            scrapeMemoryMetrics,

		scrapesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrapes_total",
			Help:      "Total number of times ProxySQL was scraped for metrics.",
		}),
		scrapeErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a ProxySQL.",
		}, []string{"collector"}),
		lastScrapeError: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from ProxySQL resulted in an error (1 for error, 0 for success).",
		}),
		lastScrapeDurationSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from ProxySQL.",
		}),
		proxysqlUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether ProxySQL is up.",
		}),
	}
}

// Describe sends the super-set of all possible descriptors of metrics collected by this Collector
// to the provided channel and returns once the last descriptor has been sent.
// Part of prometheus.Collector interface.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// We cannot know in advance what metrics the exporter will generate
	// from ProxySQL. So we use the poor man's describe method: Run a collect
	// and send the descriptors of all the collected metrics. The problem
	// here is that we need to connect to the ProxySQL. If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored ProxySQL instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect is called by the Prometheus registry when collecting metrics.
// Part of prometheus.Collector interface.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	e.scrapesTotal.Collect(ch)
	e.scrapeErrorsTotal.Collect(ch)
	e.lastScrapeError.Collect(ch)
	e.lastScrapeDurationSeconds.Collect(ch)
	e.proxysqlUp.Collect(ch)
}

func (e *Exporter) db() (*sql.DB, error) {
	db, err := sql.Open("mysql", e.dsn)
	if err == nil {
		err = db.Ping()
	}
	return db, err
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.scrapesTotal.Inc()
	var err error
	defer func(begun time.Time) {
		e.lastScrapeDurationSeconds.Set(time.Since(begun).Seconds())
		if err == nil {
			e.lastScrapeError.Set(0)
		} else {
			e.lastScrapeError.Set(1)
		}
	}(time.Now())

	db, err := e.db()
	if db != nil {
		defer db.Close()
	}
	if err != nil {
		log.Errorln("Error opening connection to ProxySQL:", err)
		e.proxysqlUp.Set(0)
		return
	}
	e.proxysqlUp.Set(1)

	if e.scrapeMySQLGlobal {
		if err = scrapeMySQLGlobal(db, ch); err != nil {
			log.Errorln("Error scraping for collect.mysql_status:", err)
			e.scrapeErrorsTotal.WithLabelValues("collect.mysql_status").Inc()
		}
	}
	if e.scrapeMySQLConnectionPool {
		if err = scrapeMySQLConnectionPool(db, ch); err != nil {
			log.Errorln("Error scraping for collect.mysql_connection_pool:", err)
			e.scrapeErrorsTotal.WithLabelValues("collect.mysql_connection_pool").Inc()
		}
	}
	if e.scrapeMySQLConnectionList {
		if err = scrapeMySQLConnectionList(db, ch); err != nil {
			log.Errorln("Error scraping for collect.mysql_connection_list:", err)
			e.scrapeErrorsTotal.WithLabelValues("collect.mysql_connection_list").Inc()
		}
	}
	if e.scrapeDetailedMySQLProcessList {
		if err = scrapeDetailedMySQLConnectionList(db, ch); err != nil {
			log.Errorln("Error scraping for collect.stats_mysql_processlist", err)
			e.scrapeErrorsTotal.WithLabelValues("collect.stats_mysql_processlist").Inc()
		}
	}
	if e.scrapeMemoryMetrics {
		if err = scrapeMemoryMetrics(db, ch); err != nil {
			log.Errorln("Error scraping for collect.stats_memory_metrics", err)
			e.scrapeErrorsTotal.WithLabelValues("collect.stats_memory_metrics").Inc()
		}
	}
}

// metric contains information about Prometheus metric.
type metric struct {
	name      string
	valueType prometheus.ValueType
	help      string
}

const mySQLGlobalQuery = "SELECT Variable_Name, Variable_Value FROM stats_mysql_global"

// https://github.com/sysown/proxysql/blob/master/doc/admin_tables.md#stats_mysql_global
// key - variable name in lowercase.
var mySQLGlobalMetrics = map[string]*metric{
	"active_transactions": {"active_transactions", prometheus.GaugeValue,
		"Current number of active transactions."},
	"client_connections_aborted": {"client_connections_aborted", prometheus.CounterValue,
		"Total number of frontend connections aborted due to invalid credential or max_connections reached."},
	"client_connections_connected": {"client_connections_connected", prometheus.GaugeValue,
		"Current number of frontend connections."},
	"client_connections_created": {"client_connections_created", prometheus.CounterValue,
		"Total number of frontend connections created so far."},
	"client_connections_non_idle": {"client_connections_non_idle", prometheus.GaugeValue,
		"Current number of client connections that are not idle."},
	"proxysql_uptime": {"proxysql_uptime", prometheus.CounterValue,
		"Uptime in seconds."},
	"questions": {"questions", prometheus.CounterValue,
		"Total number of queries sent from frontends."},
	"slow_queries": {"slow_queries", prometheus.CounterValue,
		"Total number of queries that ran for longer than the threshold in milliseconds defined in global variable mysql-long_query_time."},
}

// scrapeMySQLGlobal collects metrics from `stats_mysql_global`.
func scrapeMySQLGlobal(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(mySQLGlobalQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var name, valueS string
	for rows.Next() {
		if err = rows.Scan(&name, &valueS); err != nil {
			return err
		}
		value, err := strconv.ParseFloat(valueS, 64)
		if err != nil {
			log.Debugf("variable %s: %s", name, err)
			continue
		}

		name = strings.ToLower(name)
		m := mySQLGlobalMetrics[name]
		if m == nil {
			m = &metric{
				name:      name,
				valueType: prometheus.UntypedValue,
				help:      "Undocumented stats_mysql_global metric.",
			}
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mysql_status", m.name),
				m.help,
				nil, nil,
			),
			m.valueType, value,
		)
	}
	return rows.Err()
}

const mySQLconnectionPoolQuery = "SELECT hostgroup, srv_host, srv_port, * FROM stats_mysql_connection_pool"

// https://github.com/sysown/proxysql/blob/master/doc/admin_tables.md#stats_mysql_connection_pool
// key - column name in lowercase.
var mySQLconnectionPoolMetrics = map[string]*metric{
	"status": {"status", prometheus.GaugeValue,
		"The status of the backend server (1 - ONLINE, 2 - SHUNNED, 3 - OFFLINE_SOFT, 4 - OFFLINE_HARD)."},
	"connused": {"conn_used", prometheus.GaugeValue,
		"How many connections are currently used by ProxySQL for sending queries to the backend server."},
	"connfree": {"conn_free", prometheus.GaugeValue,
		"How many connections are currently free."},
	"connok": {"conn_ok", prometheus.CounterValue,
		"How many connections were established successfully."},
	"connerr": {"conn_err", prometheus.CounterValue,
		"How many connections weren't established successfully."},
	"queries": {"queries", prometheus.CounterValue,
		"The number of queries routed towards this particular backend server."},
	"bytes_data_sent": {"bytes_data_sent", prometheus.CounterValue,
		"The amount of data sent to the backend, excluding metadata."},
	"bytes_data_recv": {"bytes_data_recv", prometheus.CounterValue,
		"the amount of data received from the backend, excluding metadata."},

	// This column is called `Latency_us` since v1.3.1 and v1.4.0, `Latency_ms` before that,
	// but actual unit is always μs (microseconds). https://github.com/sysown/proxysql/issues/882
	// We always expose it as `latency_us`.
	"latency_us": {"latency_us", prometheus.GaugeValue,
		"The currently ping time in microseconds, as reported from Monitor."},
	"latency_ms": {"latency_us", prometheus.GaugeValue,
		"The currently ping time in microseconds, as reported from Monitor."},
}

// scrapeMySQLConnectionPool collects metrics from `stats_mysql_connection_pool`.
func scrapeMySQLConnectionPool(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(mySQLconnectionPoolQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// first 3 columns are fixed in our SELECT statement
	scan := make([]interface{}, len(columns))
	var hostgroup, srvHost, srvPort string
	scan[0], scan[1], scan[2] = &hostgroup, &srvHost, &srvPort
	for i := 3; i < len(scan); i++ {
		scan[i] = new(string)
	}

	var value float64
	var valueS, column string
	for rows.Next() {
		if err = rows.Scan(scan...); err != nil {
			return err
		}

		for i := 3; i < len(columns); i++ {
			valueS = *(scan[i].(*string))
			column = strings.ToLower(columns[i])
			switch column {
			case "hostgroup", "srv_host", "srv_port":
				continue
			case "status":
				switch valueS {
				case "ONLINE":
					value = 1
				case "SHUNNED":
					value = 2
				case "OFFLINE_SOFT":
					value = 3
				case "OFFLINE_HARD":
					value = 4
				}
			default:
				// We could use rows.ColumnTypes() when mysql driver supports them:
				//   https://github.com/go-sql-driver/mysql/issues/595
				// For now, we assume every other value is a float.
				value, err = strconv.ParseFloat(valueS, 64)
				if err != nil {
					log.Debugf("column %s: %s", column, err)
					continue
				}
			}

			m := mySQLconnectionPoolMetrics[column]
			if m == nil {
				m = &metric{
					name:      column,
					valueType: prometheus.UntypedValue,
					help:      "Undocumented stats_mysql_connection_pool metric.",
				}
			}
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "connection_pool", m.name),
					m.help,
					[]string{"hostgroup", "endpoint"}, nil,
				),
				m.valueType, value,
				hostgroup, srvHost+":"+srvPort,
			)
		}
	}
	return rows.Err()
}

const mySQLConnectionListQuery = "SELECT COUNT(cli_host) as connection_count, cli_host FROM stats_mysql_processlist GROUP BY cli_host"

var mySQLconnectionListMetrics = map[string]*metric{
	"connection_count": {"client_connection_list", prometheus.GaugeValue,
		"Total number of frontend connections"},
}

// scrapeMySQLConnectionList collects connection list from `stats_mysql_processlist`.
func scrapeMySQLConnectionList(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(mySQLConnectionListQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	scan := make([]interface{}, len(columns))
	var cliHost string
	var connNum float64

	scan[0], scan[1] = &connNum, &cliHost

	for rows.Next() {
		if err = rows.Scan(scan...); err != nil {
			return err
		}

		column := strings.ToLower(columns[0])

		m := mySQLconnectionListMetrics[column]
		if m == nil {
			m = &metric{
				name:      "client_connection_list",
				valueType: prometheus.UntypedValue,
				help:      "Undocumented stats_mysql_processlist metric.",
			}
		}

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "processlist", m.name),
				m.help,
				[]string{"client_host"}, nil,
			),
			m.valueType, connNum,
			cliHost,
		)
	}
	return rows.Err()
}

const detailedMySQLProcessListQuery = "SELECT user, db, cli_host, hostgroup, COUNT(*) as count from stats_mysql_processlist group by user, db, cli_host, hostgroup"

var detailedMySQLProcessListMetrics = map[string]*metric{
	"detailed_connection_count": {name: "detailed_client_connection_count", valueType: prometheus.GaugeValue, help: "Number of client connections per user, db, host and hostgroup."},
}

type processListResult struct {
	user, db, clientHost, hostGroup string
	count                           float64
}

func scrapeDetailedMySQLConnectionList(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(detailedMySQLProcessListQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var res processListResult

		err := rows.Scan(&res.user, &res.db, &res.clientHost, &res.hostGroup, &res.count)
		if err != nil {
			return err
		}

		m := detailedMySQLProcessListMetrics["detailed_connection_count"]

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "processlist", m.name),
				m.help,
				[]string{"user", "db", "client_host", "hostgroup"},
				nil,
			),
			m.valueType,
			res.count,
			res.user, res.db, res.clientHost, res.hostGroup,
		)
	}

	return rows.Err()
}

const memoryMetricsQuery = "select Variable_Name, Variable_Value  from stats_memory_metrics"

var memoryMetricsMetrics = map[string]*metric{
	"jemalloc_allocated": {
		name:      "jemalloc_allocated",
		valueType: prometheus.GaugeValue,
		help:      "bytes allocated by the application",
	},
	"jemalloc_active": {
		name:      "jemalloc_active",
		valueType: prometheus.GaugeValue,
		help:      "bytes in pages allocated by the application",
	},
	"jemalloc_mapped": {
		name:      "jemalloc_mapped",
		valueType: prometheus.GaugeValue,
		help:      "bytes in extents mapped by the allocator",
	},
	"jemalloc_metadata": {
		name:      "jemalloc_metadata",
		valueType: prometheus.GaugeValue,
		help:      "bytes dedicated to metadata",
	},
	"jemalloc_resident": {
		name:      "jemalloc_resident",
		valueType: prometheus.GaugeValue,
		help:      "bytes in physically resident data pages mapped by the allocator",
	},
	"auth_memory": {
		name:      "auth_memory",
		valueType: prometheus.GaugeValue,
		help:      "memory used by the authentication module to store user credentials and attributes",
	},
	"sqlite3_memory_bytes": {
		name:      "sqlite3_memory_bytes",
		valueType: prometheus.GaugeValue,
		help:      "memory used by the embedded SQLite",
	},
	"query_digest_memory": {
		name:      "query_digest_memory",
		valueType: prometheus.GaugeValue,
		help:      "memory used to store data related to stats_mysql_query_digest",
	},
}

type memoryMetricsResult struct {
	name  string
	value float64
}

func scrapeMemoryMetrics(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(memoryMetricsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var res memoryMetricsResult

		err := rows.Scan(&res.name, &res.value)
		if err != nil {
			return err
		}

		m := memoryMetricsMetrics[strings.ToLower(res.name)]
		if m == nil {
			m = &metric{
				name:      res.name,
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
			res.value,
		)
	}

	return rows.Err()
}

// check interface
var _ prometheus.Collector = (*Exporter)(nil)
