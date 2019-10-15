package collector

import (
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func openTestProxySQL(tb testing.TB) *sql.DB {
	setupTestEnv(tb)

	db, err := sql.Open("mysql", "proxysql-admin:proxysql-admin@tcp(127.0.0.1:6032)/")
	require.NoError(tb, err)
	err = db.Ping()
	require.NoError(tb, err)
	return db
}

var setupTestEnvOnce sync.Once

func setupTestEnv(tb testing.TB) {
	setupTestEnvOnce.Do(func() {
		setupMaster(tb)
		setupSlave(tb)
		setupProxySQL(tb)
	})
}

func waitFor(tb testing.TB, dsn string) *sql.DB {
	var db *sql.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			return db
		}

		if db != nil {
			db.Close()
		}
		tb.Log(err)
		time.Sleep(time.Second)
	}

	require.NoError(tb, err)
	return nil
}

func setupMaster(tb testing.TB) {
	db := waitFor(tb, "root@tcp(127.0.0.1:3307)/")
	defer db.Close()

	for _, q := range strings.Split(`
		RESET MASTER;

		CREATE USER 'replica'@'%' IDENTIFIED BY 'replica';
		GRANT REPLICATION SLAVE ON *.* TO 'replica'@'%';
	`, "\n") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		_, err := db.Exec(q)
		require.NoError(tb, err)
	}
}

func setupSlave(tb testing.TB) {
	db := waitFor(tb, "root@tcp(127.0.0.1:3308)/")
	defer db.Close()

	for _, q := range strings.Split(`
		STOP SLAVE;
		RESET MASTER, SLAVE;

		CREATE USER 'replica'@'%' IDENTIFIED BY 'replica';
		GRANT REPLICATION SLAVE ON *.* TO 'replica'@'%';

		CHANGE MASTER TO master_host = 'master', master_port = 3306, master_user = 'replica', master_password = 'replica';
		START SLAVE;
	`, "\n") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		_, err := db.Exec(q)
		require.NoError(tb, err)
	}
}

func setupProxySQL(tb testing.TB) {
	db := waitFor(tb, "proxysql-admin:proxysql-admin@tcp(127.0.0.1:6032)/")
	defer db.Close()

	for _, q := range strings.Split(`
		DELETE FROM mysql_servers;
		INSERT INTO mysql_servers(hostgroup_id, hostname, port) VALUES (1, 'master', 3306);
		INSERT INTO mysql_servers(hostgroup_id, hostname, port) VALUES (1, 'slave', 3306);
		LOAD MYSQL SERVERS TO RUNTIME;
		SAVE MYSQL SERVERS TO DISK;

		DELETE FROM mysql_users;
		INSERT INTO mysql_users(username, password, default_hostgroup) VALUES ('root', '', 1);
		INSERT INTO mysql_users(username, password, default_hostgroup) VALUES ('monitor', 'monitor', 1);
		LOAD MYSQL USERS TO RUNTIME;
		SAVE MYSQL USERS TO DISK;
	`, "\n") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		_, err := db.Exec(q)
		require.NoError(tb, err)
	}
}
