// Copyright 2016-2019 Percona LLC
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
	"errors"
	"strings"
	"testing"

	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func sanitizeQuery(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	q = strings.Replace(q, "(", "\\(", -1)
	q = strings.Replace(q, ")", "\\)", -1)
	q = strings.Replace(q, "*", "\\*", -1)
	return q
}

func TestScrapeMySQLGlobal(t *testing.T) {
	convey.Convey("Metrics are lowercase", t, convey.FailureContinues, func(cv convey.C) {
		for c, m := range mySQLGlobalMetrics {
			cv.So(c, convey.ShouldEqual, strings.ToLower(c))
			cv.So(m.name, convey.ShouldEqual, strings.ToLower(m.name))
		}
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Variable_Name", "Variable_Value"}
	rows := sqlmock.NewRows(columns).
		AddRow("Active_Transactions", "3").
		AddRow("Backend_query_time_nsec", "76355784684851").
		AddRow("Client_Connections_aborted", "0").
		AddRow("Client_Connections_connected", "64").
		AddRow("Client_Connections_created", "1087931").
		AddRow("Servers_table_version", "2019470")
	mock.ExpectQuery(mySQLGlobalQuery).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeMySQLGlobal(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []helpers.Metric{{
		"proxysql_mysql_status_active_transactions",
		"Current number of active transactions.",
		prometheus.Labels{}, dto.MetricType_GAUGE, 3,
	}, {
		"proxysql_mysql_status_backend_query_time_nsec",
		"Undocumented stats_mysql_global metric.",
		prometheus.Labels{}, dto.MetricType_UNTYPED, 76355784684851,
	}, {
		"proxysql_mysql_status_client_connections_aborted",
		"Total number of frontend connections aborted due to invalid credential or max_connections reached.",
		prometheus.Labels{}, dto.MetricType_COUNTER, 0,
	}, {
		"proxysql_mysql_status_client_connections_connected",
		"Current number of frontend connections.",
		prometheus.Labels{}, dto.MetricType_GAUGE, 64,
	}, {
		"proxysql_mysql_status_client_connections_created",
		"Total number of frontend connections created so far.",
		prometheus.Labels{}, dto.MetricType_COUNTER, 1087931,
	}, {
		"proxysql_mysql_status_servers_table_version",
		"Undocumented stats_mysql_global metric.",
		prometheus.Labels{}, dto.MetricType_UNTYPED, 2019470,
	}}
	convey.Convey("Metrics comparison", t, convey.FailureContinues, func(cv convey.C) {
		for _, expect := range counterExpected {
			got := *helpers.ReadMetric(<-ch)
			cv.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeMySQLGlobalError(t *testing.T) {
	db1, mock1, err1 := sqlmock.New()
	if err1 != nil {
		t.Fatalf("error opening a stub database connection: %s", err1)
	}
	defer db1.Close()

	mock1.ExpectQuery(mySQLGlobalQuery).WillReturnError(errors.New("an error"))
	ch1 := make(chan prometheus.Metric)

	go func() {
		scrapeMySQLGlobal(db1, ch1)
		close(ch1)
	}()

	db2, mock2, err2 := sqlmock.New()
	if err2 != nil {
		t.Fatalf("error opening a stub database connection: %s", err2)
	}
	defer db2.Close()

	columns := []string{"Variable_Name", "Variable_Value"}
	rows := sqlmock.NewRows(columns).AddRow("Active_Transactions", "3")
	mock2.ExpectQuery(sanitizeQuery(mySQLGlobalQuery)).WillReturnRows(rows)

	ch2 := make(chan prometheus.Metric)
	go func() {
		scrapeMySQLGlobal(db2, ch2)
		close(ch2)
	}()

	_ = *helpers.ReadMetric(<-ch2)
}

func TestScrapeMySQLConnectionPool(t *testing.T) {
	convey.Convey("Metrics are lowercase", t, convey.FailureContinues, func(cv convey.C) {
		for c, m := range mySQLconnectionPoolMetrics {
			cv.So(c, convey.ShouldEqual, strings.ToLower(c))
			cv.So(m.name, convey.ShouldEqual, strings.ToLower(m.name))
		}
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"hostgroup", "srv_host", "srv_port", "status", "ConnUsed", "ConnFree", "ConnOK", "ConnERR",
		"Queries", "Bytes_data_sent", "Bytes_data_recv", "Latency_us"}
	rows := sqlmock.NewRows(columns).
		AddRow("0", "10.91.142.80", "3306", "ONLINE", "0", "45", "1895677", "46", "197941647", "10984550806", "321063484988", "163").
		AddRow("0", "10.91.142.82", "3306", "SHUNNED", "0", "97", "39859", "0", "386686994", "21643682247", "641406745151", "255").
		AddRow("1", "10.91.142.88", "3306", "OFFLINE_SOFT", "0", "18", "31471", "6391", "255993467", "14327840185", "420795691329", "283").
		AddRow("2", "10.91.142.89", "3306", "OFFLINE_HARD", "0", "18", "31471", "6391", "255993467", "14327840185", "420795691329", "283")
	mock.ExpectQuery(sanitizeQuery(mySQLconnectionPoolQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeMySQLConnectionPool(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []helpers.Metric{
		{"proxysql_connection_pool_status", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_GAUGE, 1},
		{"proxysql_connection_pool_conn_used", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_GAUGE, 0},
		{"proxysql_connection_pool_conn_free", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_GAUGE, 45},
		{"proxysql_connection_pool_conn_ok", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_COUNTER, 1895677},
		{"proxysql_connection_pool_conn_err", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_COUNTER, 46},
		{"proxysql_connection_pool_queries", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_COUNTER, 197941647},
		{"proxysql_connection_pool_bytes_data_sent", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_COUNTER, 10984550806},
		{"proxysql_connection_pool_bytes_data_recv", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_COUNTER, 321063484988},
		{"proxysql_connection_pool_latency_us", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.80:3306"}, dto.MetricType_GAUGE, 163},

		{"proxysql_connection_pool_status", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_GAUGE, 2},
		{"proxysql_connection_pool_conn_used", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_GAUGE, 0},
		{"proxysql_connection_pool_conn_free", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_GAUGE, 97},
		{"proxysql_connection_pool_conn_ok", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_COUNTER, 39859},
		{"proxysql_connection_pool_conn_err", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_COUNTER, 0},
		{"proxysql_connection_pool_queries", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_COUNTER, 386686994},
		{"proxysql_connection_pool_bytes_data_sent", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_COUNTER, 21643682247},
		{"proxysql_connection_pool_bytes_data_recv", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_COUNTER, 641406745151},
		{"proxysql_connection_pool_latency_us", "", prometheus.Labels{"hostgroup": "0", "endpoint": "10.91.142.82:3306"}, dto.MetricType_GAUGE, 255},

		{"proxysql_connection_pool_status", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_GAUGE, 3},
		{"proxysql_connection_pool_conn_used", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_GAUGE, 0},
		{"proxysql_connection_pool_conn_free", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_GAUGE, 18},
		{"proxysql_connection_pool_conn_ok", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_COUNTER, 31471},
		{"proxysql_connection_pool_conn_err", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_COUNTER, 6391},
		{"proxysql_connection_pool_queries", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_COUNTER, 255993467},
		{"proxysql_connection_pool_bytes_data_sent", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_COUNTER, 14327840185},
		{"proxysql_connection_pool_bytes_data_recv", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_COUNTER, 420795691329},
		{"proxysql_connection_pool_latency_us", "", prometheus.Labels{"hostgroup": "1", "endpoint": "10.91.142.88:3306"}, dto.MetricType_GAUGE, 283},

		{"proxysql_connection_pool_status", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_GAUGE, 4},
		{"proxysql_connection_pool_conn_used", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_GAUGE, 0},
		{"proxysql_connection_pool_conn_free", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_GAUGE, 18},
		{"proxysql_connection_pool_conn_ok", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_COUNTER, 31471},
		{"proxysql_connection_pool_conn_err", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_COUNTER, 6391},
		{"proxysql_connection_pool_queries", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_COUNTER, 255993467},
		{"proxysql_connection_pool_bytes_data_sent", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_COUNTER, 14327840185},
		{"proxysql_connection_pool_bytes_data_recv", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_COUNTER, 420795691329},
		{"proxysql_connection_pool_latency_us", "", prometheus.Labels{"hostgroup": "2", "endpoint": "10.91.142.89:3306"}, dto.MetricType_GAUGE, 283},
	}
	convey.Convey("Metrics comparison", t, convey.FailureContinues, func(cv convey.C) {
		for _, expect := range counterExpected {
			got := *helpers.ReadMetric(<-ch)
			got.Help = ""
			cv.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeMySQLConnectionPoolError(t *testing.T) {
	db1, mock1, err1 := sqlmock.New()
	if err1 != nil {
		t.Fatalf("error opening a stub database connection: %s", err1)
	}
	defer db1.Close()

	mock1.ExpectQuery(mySQLconnectionPoolQuery).WillReturnError(errors.New("an error"))
	ch1 := make(chan prometheus.Metric)

	go func() {
		scrapeMySQLConnectionPool(db1, ch1)
		close(ch1)
	}()

	mySQLconnectionPoolMetrics = map[string]*metric{
		"hostgroup": {},
		"latency_us": {"latency_us", prometheus.GaugeValue,
			"The currently ping time in microseconds, as reported from Monitor."},
		"latency_ms": {"latency_us", prometheus.GaugeValue,
			"The currently ping time in microseconds, as reported from Monitor."},
	}

	db2, mock2, err2 := sqlmock.New()
	if err2 != nil {
		t.Fatalf("error opening a stub database connection: %s", err2)
	}
	defer db2.Close()

	columns := []string{"hostgroup", "srv_host", "srv_port", "status", "ConnUsed", "ConnFree", "ConnOK", "ConnERR",
		"Queries", "Bytes_data_sent", "Bytes_data_recv", "Latency_us"}
	rows := sqlmock.NewRows(columns).AddRow("0", "10.91.142.80", "3306", "ONLINE", "0", "45", "1895677", "46", "197941647", "10984550806", "321063484988", "163")
	mock2.ExpectQuery(sanitizeQuery(mySQLconnectionPoolQuery)).WillReturnRows(rows)

	ch2 := make(chan prometheus.Metric)
	go func() {
		scrapeMySQLConnectionPool(db2, ch2)
		close(ch2)
	}()

	_ = *helpers.ReadMetric(<-ch2)
}

func TestScrapeMySQLConnectionList(t *testing.T) {
	convey.Convey("Metrics are lowercase", t, convey.FailureContinues, func(cv convey.C) {
		for c, m := range mySQLconnectionListMetrics {
			cv.So(c, convey.ShouldEqual, strings.ToLower(c))
			cv.So(m.name, convey.ShouldEqual, strings.ToLower(m.name))
		}
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"connection_count", "cli_host"}
	rows := sqlmock.NewRows(columns).
		AddRow("10", "10.91.142.80").
		AddRow("15", "10.91.142.82").
		AddRow("20", "10.91.142.88").
		AddRow("25", "10.91.142.89")
	mock.ExpectQuery(sanitizeQuery(mySQLConnectionListQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeMySQLConnectionList(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []helpers.Metric{
		{"proxysql_processlist_client_connection_list", "", prometheus.Labels{"client_host": "10.91.142.80"}, dto.MetricType_GAUGE, 10},
		{"proxysql_processlist_client_connection_list", "", prometheus.Labels{"client_host": "10.91.142.82"}, dto.MetricType_GAUGE, 15},
		{"proxysql_processlist_client_connection_list", "", prometheus.Labels{"client_host": "10.91.142.88"}, dto.MetricType_GAUGE, 20},
		{"proxysql_processlist_client_connection_list", "", prometheus.Labels{"client_host": "10.91.142.89"}, dto.MetricType_GAUGE, 25},
	}

	convey.Convey("Metrics comparison", t, convey.FailureContinues, func(cv convey.C) {
		for _, expect := range counterExpected {
			got := *helpers.ReadMetric(<-ch)
			got.Help = ""
			cv.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeMySQLConnectionListError(t *testing.T) {
	db1, mock1, err1 := sqlmock.New()
	if err1 != nil {
		t.Fatalf("error opening a stub database connection: %s", err1)
	}
	defer db1.Close()

	mock1.ExpectQuery(mySQLConnectionListQuery).WillReturnError(errors.New("an error"))
	ch1 := make(chan prometheus.Metric)

	go func() {
		scrapeMySQLConnectionList(db1, ch1)
		close(ch1)
	}()

	mySQLconnectionListMetrics = map[string]*metric{
		"client_connection_list": {},
	}

	db2, mock2, err2 := sqlmock.New()
	if err2 != nil {
		t.Fatalf("error opening a stub database connection: %s", err2)
	}
	defer db2.Close()

	columns := []string{"connection_count", "cli_host"}
	rows := sqlmock.NewRows(columns).AddRow("10", "10.91.142.80")
	mock2.ExpectQuery(sanitizeQuery(mySQLConnectionListQuery)).WillReturnRows(rows)

	ch2 := make(chan prometheus.Metric)
	go func() {
		scrapeMySQLConnectionList(db2, ch2)
		close(ch2)
	}()

	_ = *helpers.ReadMetric(<-ch2)
}

func TestScrapeDetailedConnectionList(t *testing.T) {
	convey.Convey("Metrics are lowercase", t, convey.FailureContinues, func(cv convey.C) {
		for c, m := range detailedMySQLProcessListMetrics {
			cv.So(c, convey.ShouldEqual, strings.ToLower(c))
			cv.So(m.name, convey.ShouldEqual, strings.ToLower(m.name))
		}
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"user", "db", "cli_host", "hostgroup", "count"}
	rows := sqlmock.NewRows(columns).
		AddRow("user_1", "database_1", "10.91.142.80", "1001", 1).
		AddRow("user_2", "database_2", "10.91.142.82", "1002", 2).
		AddRow("user_3", "database_3", "10.91.142.88", "1003", 3).
		AddRow("user_4", "database_4", "10.91.142.89", "1004", 4)
	mock.ExpectQuery(sanitizeQuery(detailedMySQLProcessListQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeDetailedMySQLConnectionList(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []helpers.Metric{{
		"proxysql_processlist_detailed_client_connection_count",
		"Number of client connections per user, db, host and hostgroup.",
		prometheus.Labels{"client_host": "10.91.142.80", "user": "user_1", "db": "database_1", "hostgroup": "1001"},
		dto.MetricType_GAUGE, 1,
	}, {
		"proxysql_processlist_detailed_client_connection_count",
		"Number of client connections per user, db, host and hostgroup.",
		prometheus.Labels{"client_host": "10.91.142.82", "user": "user_2", "db": "database_2", "hostgroup": "1002"},
		dto.MetricType_GAUGE, 2,
	}, {
		"proxysql_processlist_detailed_client_connection_count",
		"Number of client connections per user, db, host and hostgroup.",
		prometheus.Labels{"client_host": "10.91.142.88", "user": "user_3", "db": "database_3", "hostgroup": "1003"},
		dto.MetricType_GAUGE, 3,
	}, {
		"proxysql_processlist_detailed_client_connection_count",
		"Number of client connections per user, db, host and hostgroup.",
		prometheus.Labels{"client_host": "10.91.142.89", "user": "user_4", "db": "database_4", "hostgroup": "1004"},
		dto.MetricType_GAUGE, 4,
	}}

	convey.Convey("Metrics comparison", t, convey.FailureContinues, func(cv convey.C) {
		for _, expect := range counterExpected {
			got := *helpers.ReadMetric(<-ch)
			cv.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeCommands(t *testing.T) {
	convey.Convey("Metrics are lowercase", t, convey.FailureContinues, func(cv convey.C) {
		for c, m := range mysqlCommandCountersMetrics {
			cv.So(c, convey.ShouldEqual, strings.ToLower(c))
			cv.So(m.name, convey.ShouldEqual, strings.ToLower(m.name))
		}
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Command", "Total_Time_us", "Total_cnt", "cnt_100us", "cnt_500us", "cnt_1ms", "cnt_5ms", "cnt_10ms", "cnt_50ms", "cnt_100ms", "cnt_500ms", "cnt_1s", "cnt_5s", "cnt_10s", "cnt_INFs"}
	rows := sqlmock.NewRows(columns).
		AddRow("SELECT", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0).
		AddRow("DELETE", 10, 85, 41, 23, 15, 10, 0, 48, 10456, 20, 6548990, 1230, 7890, 56978940)
	mock.ExpectQuery(sanitizeQuery(mysqlCommandCounterQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeMysqlCommandCounters(db, ch); err != nil {
			t.Errorf("error calling function on test %s", err)
		}
		close(ch)
	}()

	counterExpected := []helpers.Metric{
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "100us"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "500us"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "1ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "5ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "10ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "50ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "100ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "500ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "1s"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "5s"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "10s"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "INFs"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "SELECT", "time_spent": "Total"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_total_time", "", prometheus.Labels{"command": "SELECT"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "100us"}, dto.MetricType_COUNTER, 41},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "500us"}, dto.MetricType_COUNTER, 23},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "1ms"}, dto.MetricType_COUNTER, 15},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "5ms"}, dto.MetricType_COUNTER, 10},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "10ms"}, dto.MetricType_COUNTER, 0},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "50ms"}, dto.MetricType_COUNTER, 48},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "100ms"}, dto.MetricType_COUNTER, 10456},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "500ms"}, dto.MetricType_COUNTER, 20},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "1s"}, dto.MetricType_COUNTER, 6548990},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "5s"}, dto.MetricType_COUNTER, 1230},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "10s"}, dto.MetricType_COUNTER, 7890},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "INFs"}, dto.MetricType_COUNTER, 56978940},
		{"proxysql_command_count", "", prometheus.Labels{"command": "DELETE", "time_spent": "Total"}, dto.MetricType_COUNTER, 85},
		{"proxysql_command_total_time", "", prometheus.Labels{"command": "DELETE"}, dto.MetricType_COUNTER, 10},
	}

	convey.Convey("Metrics comparison", t, convey.FailureContinues, func(cv convey.C) {
		for _, expect := range counterExpected {
			got := *helpers.ReadMetric(<-ch)
			got.Help = ""
			cv.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeMemoryMetrics(t *testing.T) {
	convey.Convey("Metrics are lowercase", t, convey.FailureContinues, func(cv convey.C) {
		for c, m := range memoryMetricsMetrics {
			cv.So(c, convey.ShouldEqual, strings.ToLower(c))
			cv.So(m.name, convey.ShouldEqual, strings.ToLower(m.name))
		}
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Variable_Name", "Variable_Value"}
	rows := sqlmock.NewRows(columns).
		AddRow("stack_memory_admin_threads", "32541").
		AddRow("query_digest_memory", "7314")
	mock.ExpectQuery(sanitizeQuery(memoryMetricsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeMemoryMetrics(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []helpers.Metric{
		{"proxysql_stats_memory_stack_memory_admin_threads", "Undocumented stats_memory_metrics metric.", prometheus.Labels{}, dto.MetricType_UNTYPED, 32541},
		{"proxysql_stats_memory_query_digest_memory", "memory used to store data related to stats_mysql_query_digest", prometheus.Labels{}, dto.MetricType_GAUGE, 7314},
	}

	convey.Convey("Metrics comparison", t, convey.FailureContinues, func(cv convey.C) {
		for _, expect := range counterExpected {
			got := *helpers.ReadMetric(<-ch)
			cv.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeMemoryMetricsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(sanitizeQuery(memoryMetricsQuery)).WillReturnError(errors.New("error"))

	ch := make(chan prometheus.Metric)
	err = scrapeMemoryMetrics(db, ch)
	assert.Error(t, err)
}

func TestScrapeDetailedConnectionLisError(t *testing.T) {
	t.Run("error on sql query", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		defer db.Close()

		mock.ExpectQuery(sanitizeQuery(detailedMySQLProcessListQuery)).WillReturnError(errors.New("error"))

		ch := make(chan prometheus.Metric)

		err = scrapeDetailedMySQLConnectionList(db, ch)
		assert.Error(t, err)
	})
}

func TestExporter(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping integration test")
	}

	setupTestEnv(t)

	// wait up to 30 seconds for ProxySQL to become available
	exporter := NewExporter("proxysql-admin:proxysql-admin@tcp(127.0.0.1:6032)/", true, true, true, true, true, true)

	convey.Convey("Metrics descriptions", t, convey.FailureContinues, func(cv convey.C) {
		ch := make(chan *prometheus.Desc)
		go func() {
			exporter.Describe(ch)
			close(ch)
		}()

		descs := make(map[string]struct{})
		for d := range ch {
			descs[d.String()] = struct{}{}
		}

		cv.So(descs, convey.ShouldContainKey,
			`Desc{fqName: "proxysql_connection_pool_latency_us", help: "The currently ping time in microseconds, as reported from Monitor.", constLabels: {}, variableLabels: [hostgroup endpoint]}`)
	})

	convey.Convey("Metrics data", t, convey.FailureContinues, func(cv convey.C) {
		ch := make(chan prometheus.Metric)
		go func() {
			exporter.Collect(ch)
			close(ch)
		}()

		var metrics []helpers.Metric
		for m := range ch {
			got := *helpers.ReadMetric(m)
			got.Value = 0 // ignore actual values in comparison for now
			metrics = append(metrics, got)
		}

		for _, m := range metrics {
			cv.So(m.Name, convey.ShouldEqual, strings.ToLower(m.Name))
			for k := range m.Labels {
				cv.So(k, convey.ShouldEqual, strings.ToLower(k))
			}
		}

		cv.So(helpers.Metric{"proxysql_connection_pool_latency_us", "", prometheus.Labels{"hostgroup": "1", "endpoint": "mysql:3306"}, dto.MetricType_GAUGE, 0},
			convey.ShouldBeIn, metrics)
		cv.So(helpers.Metric{"proxysql_connection_pool_latency_us", "", prometheus.Labels{"hostgroup": "1", "endpoint": "percona-server:3306"}, dto.MetricType_GAUGE, 0},
			convey.ShouldBeIn, metrics)
	})
}
