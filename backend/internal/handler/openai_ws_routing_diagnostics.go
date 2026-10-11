package handler

import (
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// 每次 Proxy 重试的局部 turn 从 1 开始；诊断沿用整条连接的逻辑 turn。
// 该映射只管理诊断，不改变既有日志、计费或重试钩子的局部编号。
type openAIWSRoutingTurns struct {
	mu      sync.Mutex
	current int
	start   int
}

func (s *openAIWSRoutingTurns) beginProxy(c *gin.Context) {
	s.mu.Lock()
	if s.current == 0 {
		s.current = 1
	}
	s.start = s.current
	turn := s.current
	s.mu.Unlock()
	if c.Request != nil {
		c.Request = c.Request.WithContext(service.EnsureServiceStatusTurn(service.EnsureRoutingDiagnosticsTurn(c.Request.Context(), turn), turn))
	}
}

func (s *openAIWSRoutingTurns) beginTurn(c *gin.Context, localTurn int) {
	s.mu.Lock()
	turn := s.start + localTurn - 1
	s.current = turn
	s.mu.Unlock()
	service.BeginOpsStreamTurnWithRoutingTurn(c, localTurn, turn)
}
