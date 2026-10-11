package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 代理重启会重复局部轮次；首个失败生效的范围必须仍是连接的逻辑轮次。
func TestOpsStreamLogicalTurnDedupSurvivesProxyRestart(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportWS)

	BeginOpsStreamTurnWithRoutingTurn(c, 2, 2)
	MarkOpsStreamFailure(c, "upstream_error", "first", "logical-turn-2", 400)
	// 同一逻辑轮次换号后局部编号重启，不能再记录兜底失败。
	BeginOpsStreamTurnWithRoutingTurn(c, 1, 2)
	MarkOpsStreamFailure(c, "upstream_error", "duplicate", "same-turn-fallback", 500)
	require.Len(t, GetOpsStreamErrors(c), 1)

	// 下一逻辑轮次又使用局部编号2，仍须保留新的真实失败。
	BeginOpsStreamTurnWithRoutingTurn(c, 2, 3)
	MarkOpsStreamFailure(c, "upstream_error", "next", "logical-turn-3", 400)
	MarkOpsStreamFailure(c, "upstream_error", "duplicate", "next-turn-fallback", 500)
	got := GetOpsStreamErrors(c)
	require.Len(t, got, 2)
	require.Equal(t, "logical-turn-2", got[0].Message)
	require.Equal(t, "logical-turn-3", got[1].Message)
	require.Equal(t, 2, got[0].Turn, "保留既有转发局部轮次字段")
	require.Equal(t, 2, got[1].Turn, "保留既有转发局部轮次字段")
}
