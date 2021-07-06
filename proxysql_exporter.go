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
	"flag"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	program           = "proxysql_exporter"
	defaultDataSource = "stats:stats@tcp(localhost:6032)/"
)

var (
	versionF       = flag.Bool("version", false, "Print version information and exit.")
	listenAddressF = flag.String("web.listen-address", ":42004", "Address to listen on for web interface and telemetry.")
	telemetryPathF = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")

	mysqlStatusF                 = flag.Bool("collect.mysql_status", true, "Collect from stats_mysql_global (SHOW MYSQL STATUS).")
	mysqlConnectionPoolF         = flag.Bool("collect.mysql_connection_pool", true, "Collect from stats_mysql_connection_pool.")
	mysqlConnectionListF         = flag.Bool("collect.mysql_connection_list", true, "Collect connection list from stats_mysql_processlist.")
	mysqlDetailedConnectionListF = flag.Bool("collect.detailed.stats_mysql_processlist", false, "Collect detailed connection list from stats_mysql_processlist.")
	mysqlCommandCounter          = flag.Bool("collect.stats_command_counter", false, "Collect histograms over command latency")
	mysqlRuntimeServers          = flag.Bool("collect.runtime_mysql_servers", false, "Collect from runtime_mysql_servers.")
	memoryMetricsF               = flag.Bool("collect.stats_memory_metrics", false, "Collect memory metrics from stats_memory_metrics.")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s %s exports various ProxySQL metrics in Prometheus format.\n", os.Args[0], version.Version)
		fmt.Fprintf(os.Stderr, "It uses DATA_SOURCE_NAME environment variable with following format: https://github.com/go-sql-driver/mysql#dsn-data-source-name\n")
		fmt.Fprintf(os.Stderr, "Default value is %q.\n\n", defaultDataSource)
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionF {
		fmt.Println(version.Print(program))
		os.Exit(0)
	}

	dsn := os.Getenv("DATA_SOURCE_NAME")
	if dsn == "" {
		dsn = defaultDataSource
	}

	log.Infof("Starting %s %s for %s", program, version.Version, dsn)

	exporter := NewExporter(dsn, *mysqlStatusF, *mysqlConnectionPoolF, *mysqlConnectionListF, *mysqlDetailedConnectionListF,
		*mysqlRuntimeServers, *memoryMetricsF, *mysqlCommandCounter)
	prometheus.MustRegister(exporter)

	exporter_shared.RunServer("ProxySQL", *listenAddressF, *telemetryPathF, promhttp.Handler())
}
