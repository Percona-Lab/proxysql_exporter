package main

import (
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var setupTestEnvOnce sync.Once

func setupTestEnv(tb testing.TB) {
	setupTestEnvOnce.Do(func() {
		// wait up to 30 seconds for ProxySQL to become available
		exporter := NewExporter("proxysql-admin:proxysql-admin@tcp(127.0.0.1:6032)/", false, false, false, false, false, false)
		var db *sql.DB
		var err error
		for i := 0; i < 30; i++ {
			db, err = exporter.db()
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
		require.NoError(tb, err)
		defer db.Close()

		// configure ProxySQL
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
			_, err = db.Exec(q)
			require.NoError(tb, err)
		}
	})
}
