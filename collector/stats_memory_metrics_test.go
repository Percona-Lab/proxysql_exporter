package collector

import (
	"testing"

	"github.com/percona/exporter_shared/helpers"
	"github.com/stretchr/testify/assert"
)

func TestStatsMemoryMetricsCollector(t *testing.T) {
	db := openTestProxySQL(t)
	defer db.Close()

	c := NewStatsMemoryMetricsCollector(db)
	actual := helpers.Format(helpers.CollectMetrics(c))
	assert.Equal(t, []string{}, actual)
}
