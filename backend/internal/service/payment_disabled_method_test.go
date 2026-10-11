//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type disabledPaymentTestUserRepo struct {
	UserRepository
	user *User
}

func (r disabledPaymentTestUserRepo) GetByID(context.Context, int64) (*User, error) {
	return r.user, nil
}

type disabledPaymentTestLoadBalancer struct {
	selection *payment.InstanceSelection
	calls     int
}

func (lb *disabledPaymentTestLoadBalancer) GetInstanceConfig(context.Context, int64) (map[string]string, error) {
	return lb.selection.Config, nil
}

func (lb *disabledPaymentTestLoadBalancer) SelectInstance(context.Context, string, payment.PaymentType, payment.Strategy, float64) (*payment.InstanceSelection, error) {
	lb.calls++
	return lb.selection, nil
}

type disabledPaymentTestTransport struct {
	calls int
}

func (transport *disabledPaymentTestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls++
	return nil, errors.New("unexpected payment provider request")
}

func TestCreateOrderRejectsAirwallexBeforePaymentSideEffects(t *testing.T) {
	for _, tt := range []struct {
		name        string
		paymentType string
		wantSelects int
	}{
		{name: "direct request", paymentType: payment.TypeAirwallex},
		{name: "normalized request", paymentType: " AIRWALLEX "},
		{name: "instance selected through another method", paymentType: "legacy_air_method", wantSelects: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			user, err := client.User.Create().
				SetEmail("disabled-air@example.com").
				SetPasswordHash("hash").
				SetUsername("disabled-air").
				Save(ctx)
			require.NoError(t, err)
			config := map[string]string{
				"clientId": "test-client", "apiKey": "test-key", "webhookSecret": "test-secret",
				"apiBase": "https://api-demo.airwallex.com/api/v1",
			}
			instance, err := client.PaymentProviderInstance.Create().
				SetProviderKey(payment.TypeAirwallex).
				SetName("Historical Airwallex").
				SetConfig(`{"clientId":"test-client","apiKey":"test-key","webhookSecret":"test-secret","apiBase":"https://api-demo.airwallex.com/api/v1"}`).
				SetSupportedTypes(payment.TypeAirwallex + ",legacy_air_method").
				SetEnabled(true).
				Save(ctx)
			require.NoError(t, err)
			lb := &disabledPaymentTestLoadBalancer{selection: &payment.InstanceSelection{
				InstanceID: strconv.FormatInt(instance.ID, 10), ProviderKey: payment.TypeAirwallex, Config: config,
			}}
			transport := &disabledPaymentTestTransport{}
			originalTransport := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			svc := &PaymentService{
				entClient: client, loadBalancer: lb,
				configService: &PaymentConfigService{entClient: client, settingRepo: &paymentConfigSettingRepoStub{
					values: map[string]string{SettingPaymentEnabled: "true", SettingEnabledPaymentTypes: payment.TypeAirwallex},
				}},
				userRepo: disabledPaymentTestUserRepo{user: &User{
					ID: user.ID, Email: user.Email, Username: user.Username, Status: payment.EntityStatusActive,
				}},
			}

			resp, err := svc.CreateOrder(ctx, CreateOrderRequest{UserID: user.ID, Amount: 10, PaymentType: tt.paymentType})
			require.Nil(t, resp)
			require.Equal(t, "PAYMENT_METHOD_DISABLED", infraerrors.Reason(err))
			require.EqualValues(t, http.StatusForbidden, infraerrors.FromError(err).Code)
			require.Equal(t, tt.wantSelects, lb.calls)
			require.Zero(t, transport.calls)
			orders, err := client.PaymentOrder.Query().Count(ctx)
			require.NoError(t, err)
			require.Zero(t, orders)
			audits, err := client.PaymentAuditLog.Query().Count(ctx)
			require.NoError(t, err)
			require.Zero(t, audits)
			preserved, err := client.PaymentProviderInstance.Get(ctx, instance.ID)
			require.NoError(t, err)
			require.True(t, preserved.Enabled)
			require.Equal(t, instance.Config, preserved.Config)
		})
	}
}

func TestPaymentMethodDirectoryOmitsAirwallexAndPreservesInstances(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	_, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAirwallex).
		SetName("Historical Airwallex").
		SetConfig("{}").
		SetSupportedTypes("airwallex,legacy_air_method").
		SetEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeEasyPay).
		SetName("EasyPay").
		SetConfig("{}").
		SetSupportedTypes("alipay,airwallex").
		SetEnabled(true).
		SetLimits(`{"alipay":{"singleMin":2,"singleMax":20}}`).
		Save(ctx)
	require.NoError(t, err)
	settings := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingEnabledPaymentTypes: "alipay,airwallex,stripe",
	}}
	svc := &PaymentConfigService{entClient: client, settingRepo: settings}

	limits, err := svc.GetAvailableMethodLimits(ctx)
	require.NoError(t, err)
	require.Len(t, limits.Methods, 1)
	require.Contains(t, limits.Methods, payment.TypeAlipay)
	require.Equal(t, 2.0, limits.GlobalMin)
	require.Equal(t, 20.0, limits.GlobalMax)
	config, err := svc.GetPaymentConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{payment.TypeAlipay, payment.TypeStripe}, config.EnabledTypes)
	require.Equal(t, "alipay,airwallex,stripe", settings.values[SettingEnabledPaymentTypes])
	instances, err := client.PaymentProviderInstance.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, instances)
}
