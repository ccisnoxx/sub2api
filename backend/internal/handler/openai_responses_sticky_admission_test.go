//go:build unit

package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type responsesAdmissionBindings struct {
	testutil.StubGatewayCache
	owner  int64
	writes int
}

func (b *responsesAdmissionBindings) SetSessionAccountID(_ context.Context, _ int64, _ string, owner int64, _ time.Duration) error {
	b.owner = owner
	b.writes++
	return nil
}

type responsesAdmissionSlots struct {
	fakeConcurrencyCache
	wait     bool
	calls    int
	releases int
	waiting  int
}

func (s *responsesAdmissionSlots) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	s.calls++
	return !s.wait || s.calls > 1, nil
}

func (s *responsesAdmissionSlots) ReleaseAccountSlot(context.Context, int64, string) error {
	s.releases++
	return nil
}

func (s *responsesAdmissionSlots) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	s.waiting++
	return true, nil
}

func (s *responsesAdmissionSlots) DecrementAccountWaitCount(context.Context, int64) error {
	s.waiting--
	return nil
}

func TestResponsesStickyAdmissionPreservesTemporarySpillover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, wait := range []bool{false, true} {
		for _, preserve := range []bool{false, true} {
			name := "quick"
			if wait {
				name = "queued"
			}
			if preserve {
				name += "/spillover"
			} else {
				name += "/new_binding"
			}
			t.Run(name, func(t *testing.T) {
				bindings := &responsesAdmissionBindings{owner: 1}
				slots := &responsesAdmissionSlots{wait: wait}
				concurrency := service.NewConcurrencyService(slots)
				gateway := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, bindings, nil, nil, concurrency, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				h := &OpenAIGatewayHandler{gatewayService: gateway, concurrencyHelper: NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0)}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
				selection := &service.AccountSelectionResult{
					Account:               &service.Account{ID: 2, Platform: service.PlatformOpenAI},
					PreserveStickyBinding: preserve,
					WaitPlan:              &service.AccountWaitPlan{AccountID: 2, MaxConcurrency: 2, Timeout: time.Second, MaxWaiting: 2},
				}
				streamStarted := false
				release, result := h.acquireResponsesAccountSlot(c, nil, "session", selection, false, &streamStarted, zap.NewNop())
				require.Equal(t, openAISlotAcquireOK, result)
				require.NotNil(t, release)
				release()
				require.Equal(t, 1, slots.releases)
				require.Zero(t, slots.waiting)
				if wait {
					require.GreaterOrEqual(t, slots.calls, 2)
				}
				if preserve {
					require.Equal(t, int64(1), bindings.owner)
					require.Zero(t, bindings.writes)
				} else {
					require.Equal(t, int64(2), bindings.owner)
					require.Equal(t, 1, bindings.writes)
				}
			})
		}
	}
}
