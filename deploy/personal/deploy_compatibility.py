"""唯一审定的运行树与单向/累计兼容证明；历史身份逐字保留。"""
import re
from deploy_common import *

# 完整后端与运行资源；部署工具、来源记录不属于被部署的应用运行配置。
RUNTIME_PATHS = ["backend", "Dockerfile", "Dockerfile.goreleaser", ".dockerignore", "deploy"]
RUNTIME_EXCLUDES = ["deploy/personal", "deploy/personal-source.json"]
# 本轮审定安全补丁的完整文件字节；未来补丁须另行审定，不按路径泛化放行。
SECURITY_PATCH_ID = "go1.27.2-xnet0.60.0-http2-lint"
SECURITY_PATCH_FILES = {
    "backend/.golangci.yml": (
        "ed037798aa38c9e377aafa2fad35c12dc95b1edb29f18ec61f0a08b4bfbd5599",
        "0ee503a72db01ad77e24cdab2038b53880835ec42d6aa100028e1138c48964f8",
    ),
    "backend/go.mod": (
        "105595eed2189a9e501d17a5a046fce5b84e0830b3792c7d13479553665cd12d",
        "a1b6b1568eaed08d6395f4cc8ec61a0354c0ba0788a029e035858f89e066e78c",
    ),
    "backend/go.sum": (
        "0ed026ee0ecad45e8c31563a0da61b4f8f00f750b02e32ab159ebe947cadc105",
        "109d6d9f25fe0e39747e202035ec2074ff0da6d2da6dde3dc50b4d423491543f",
    ),
    "Dockerfile": (
        "2d87d5b38082e0bba357e64ee5fc32085c81e6aa82ced4f24e4d8e483661db72",
        "28f9453bb577d8179d5729dd0b47b285ec8a033301774ceb516493033e90c575",
    ),
    "deploy/Dockerfile": (
        "9e0c48a1e62b5f4fb17dc9e4008e962abf47535ab883ade9bb9f1063a94da32d",
        "37e39456ea90d035094ad00e881655ccd98383670ecca848bb8722090f35d489",
    ),
}


# 独立审定的 .3 → .5 单向升级；指纹包含运行路径下全部条目的路径、mode、type 与 Git object。
FORWARD_UPGRADE_ID = "klno.3-tps.1-to-klno.5"
FORWARD_OLD_SOURCE = {
    "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.3",
    "upstream_sha": "de08df02ae1d81668a22f798b398aa0438ac1276",
}
FORWARD_NEW_SOURCE = {
    "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.5",
    "upstream_sha": "c7aacf5d3ae383d0d5c75f471f66e61690a5701d",
}
FORWARD_OLD_REVISION = "896de21b4be7f4ec4b4236f4df663b47371665b0"
FORWARD_NEW_REVISION = "c00e8982258736a188a09023e901d8d39671814f"
FORWARD_OLD_RUNTIME = "ecf6eff8c2902b29a1a689232dc8a1028e9d10e5615bcb6e293d35d8d278df35"
FORWARD_NEW_RUNTIME = "928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d"
FORWARD_CHANGED_PATHS = (
    "Dockerfile",
    "backend/.golangci.yml",
    "backend/go.mod",
    "backend/go.sum",
    "backend/internal/handler/failover_loop.go",
    "backend/internal/handler/openai_chat_completions.go",
    "backend/internal/handler/openai_codex_wire_entry_test.go",
    "backend/internal/handler/openai_gateway_handler.go",
    "backend/internal/handler/openai_gwpool_429_entry_test.go",
    "backend/internal/handler/openai_gwpool_compat_retry_entry_test.go",
    "backend/internal/handler/openai_gwpool_retry_test.go",
    "backend/internal/handler/openai_gwpool_slot_wait.go",
    "backend/internal/pkg/gwpool/client.go",
    "backend/internal/pkg/gwpool/cooldown_batch.go",
    "backend/internal/pkg/gwpool/cooldown_batch_test.go",
    "backend/internal/pkg/gwpool/protocol_test.go",
    "backend/internal/repository/account_repo.go",
    "backend/internal/repository/account_repo_temp_unsched_test.go",
    "backend/internal/repository/concurrency_slot_renewal.go",
    "backend/internal/repository/concurrency_slot_renewal_test.go",
    "backend/internal/service/concurrency_service.go",
    "backend/internal/service/concurrency_slot_renewal.go",
    "backend/internal/service/concurrency_slot_renewal_test.go",
    "backend/internal/service/gateway_service.go",
    "backend/internal/service/openai_codex_bridge_wire_profile_test.go",
    "backend/internal/service/openai_codex_cookies.go",
    "backend/internal/service/openai_codex_side_calls.go",
    "backend/internal/service/openai_codex_side_calls_test.go",
    "backend/internal/service/openai_compact_sse_keepalive.go",
    "backend/internal/service/openai_compat_bridge_identity.go",
    "backend/internal/service/openai_gateway_chat_completions.go",
    "backend/internal/service/openai_gateway_forward.go",
    "backend/internal/service/openai_gateway_messages.go",
    "backend/internal/service/openai_gateway_passthrough.go",
    "backend/internal/service/openai_gateway_service.go",
    "backend/internal/service/openai_gwpool.go",
    "backend/internal/service/openai_gwpool_accounts_repo_test.go",
    "backend/internal/service/openai_gwpool_active_accounts.go",
    "backend/internal/service/openai_gwpool_active_accounts_test.go",
    "backend/internal/service/openai_gwpool_active_usage.go",
    "backend/internal/service/openai_gwpool_active_usage_test.go",
    "backend/internal/service/openai_gwpool_adaptive_rank.go",
    "backend/internal/service/openai_gwpool_adaptive_rank_test.go",
    "backend/internal/service/openai_gwpool_batch_test.go",
    "backend/internal/service/openai_gwpool_bulk_test.go",
    "backend/internal/service/openai_gwpool_business_retry.go",
    "backend/internal/service/openai_gwpool_business_retry_test.go",
    "backend/internal/service/openai_gwpool_candidate_queue.go",
    "backend/internal/service/openai_gwpool_combined_usage_test.go",
    "backend/internal/service/openai_gwpool_concurrency_test.go",
    "backend/internal/service/openai_gwpool_confirmation_boundary_test.go",
    "backend/internal/service/openai_gwpool_contact_rank.go",
    "backend/internal/service/openai_gwpool_contact_rank_test.go",
    "backend/internal/service/openai_gwpool_contacts.go",
    "backend/internal/service/openai_gwpool_continuous_wait.go",
    "backend/internal/service/openai_gwpool_continuous_wait_test.go",
    "backend/internal/service/openai_gwpool_cooldown.go",
    "backend/internal/service/openai_gwpool_cooldown_estimate.go",
    "backend/internal/service/openai_gwpool_cooldown_estimate_test.go",
    "backend/internal/service/openai_gwpool_cooldown_refresh.go",
    "backend/internal/service/openai_gwpool_cooldown_refresh_test.go",
    "backend/internal/service/openai_gwpool_cooldown_reset.go",
    "backend/internal/service/openai_gwpool_cooldown_reset_test.go",
    "backend/internal/service/openai_gwpool_cooldown_test.go",
    "backend/internal/service/openai_gwpool_display_snapshot.go",
    "backend/internal/service/openai_gwpool_early.go",
    "backend/internal/service/openai_gwpool_early_test.go",
    "backend/internal/service/openai_gwpool_echo_boundary_test.go",
    "backend/internal/service/openai_gwpool_echo_confirm.go",
    "backend/internal/service/openai_gwpool_echo_confirm_test.go",
    "backend/internal/service/openai_gwpool_exhaust_candidates_test.go",
    "backend/internal/service/openai_gwpool_feedback.go",
    "backend/internal/service/openai_gwpool_feedback_test.go",
    "backend/internal/service/openai_gwpool_first_output.go",
    "backend/internal/service/openai_gwpool_first_output_test.go",
    "backend/internal/service/openai_gwpool_full_usage.go",
    "backend/internal/service/openai_gwpool_full_usage_test.go",
    "backend/internal/service/openai_gwpool_gateway_history.go",
    "backend/internal/service/openai_gwpool_gateway_history_test.go",
    "backend/internal/service/openai_gwpool_guard_test.go",
    "backend/internal/service/openai_gwpool_inventory.go",
    "backend/internal/service/openai_gwpool_local_rank.go",
    "backend/internal/service/openai_gwpool_local_rank_test.go",
    "backend/internal/service/openai_gwpool_local_resume_test.go",
    "backend/internal/service/openai_gwpool_manual_retry.go",
    "backend/internal/service/openai_gwpool_manual_retry_test.go",
    "backend/internal/service/openai_gwpool_member_identity.go",
    "backend/internal/service/openai_gwpool_member_identity_test.go",
    "backend/internal/service/openai_gwpool_outbox.go",
    "backend/internal/service/openai_gwpool_policy_test.go",
    "backend/internal/service/openai_gwpool_post_response_state_test.go",
    "backend/internal/service/openai_gwpool_prepare.go",
    "backend/internal/service/openai_gwpool_prepare_policy.go",
    "backend/internal/service/openai_gwpool_prepare_test.go",
    "backend/internal/service/openai_gwpool_priority_test.go",
    "backend/internal/service/openai_gwpool_probe_error.go",
    "backend/internal/service/openai_gwpool_probe_flight.go",
    "backend/internal/service/openai_gwpool_probe_retirement.go",
    "backend/internal/service/openai_gwpool_probe_retirement_test.go",
    "backend/internal/service/openai_gwpool_probe_timeout.go",
    "backend/internal/service/openai_gwpool_probe_timeout_test.go",
    "backend/internal/service/openai_gwpool_protocol_test.go",
    "backend/internal/service/openai_gwpool_report_policy.go",
    "backend/internal/service/openai_gwpool_rest.go",
    "backend/internal/service/openai_gwpool_rest_display.go",
    "backend/internal/service/openai_gwpool_rest_test.go",
    "backend/internal/service/openai_gwpool_rest_timer.go",
    "backend/internal/service/openai_gwpool_resume.go",
    "backend/internal/service/openai_gwpool_resume_test.go",
    "backend/internal/service/openai_gwpool_rotation.go",
    "backend/internal/service/openai_gwpool_rotation_test.go",
    "backend/internal/service/openai_gwpool_rounds.go",
    "backend/internal/service/openai_gwpool_rounds_test.go",
    "backend/internal/service/openai_gwpool_route_deadline.go",
    "backend/internal/service/openai_gwpool_route_deadline_test.go",
    "backend/internal/service/openai_gwpool_runtime_snapshot_test.go",
    "backend/internal/service/openai_gwpool_runtime_test.go",
    "backend/internal/service/openai_gwpool_runtime_view.go",
    "backend/internal/service/openai_gwpool_selection.go",
    "backend/internal/service/openai_gwpool_single_ticket_test.go",
    "backend/internal/service/openai_gwpool_state_echo.go",
    "backend/internal/service/openai_gwpool_state_echo_test.go",
    "backend/internal/service/openai_gwpool_stream_retry.go",
    "backend/internal/service/openai_gwpool_stream_retry_test.go",
    "backend/internal/service/openai_gwpool_test.go",
    "backend/internal/service/openai_gwpool_usage_activity.go",
    "backend/internal/service/openai_gwpool_usage_activity_test.go",
    "backend/internal/service/openai_gwpool_usage_retirement_test.go",
    "backend/internal/service/openai_gwpool_usage_round.go",
    "backend/internal/service/openai_gwpool_usage_round_test.go",
    "backend/internal/service/openai_gwpool_verified_model_test.go",
    "backend/internal/service/openai_gwpool_wait.go",
    "backend/internal/service/openai_gwpool_wait_keepalive.go",
    "backend/internal/service/openai_gwpool_wait_keepalive_test.go",
    "backend/internal/service/openai_gwpool_wait_test.go",
    "backend/internal/service/openai_gwpool_waiter_inventory_test.go",
    "backend/internal/service/openai_gwpool_warm.go",
    "backend/internal/service/openai_gwpool_warm_test.go",
    "backend/internal/service/openai_gwpool_ws_retry_test.go",
    "backend/internal/service/openai_gwpool_ws_wait.go",
    "backend/internal/service/openai_gwpool_ws_wait_test.go",
    "backend/internal/service/openai_oauth_429_retry_lane.go",
    "backend/internal/service/openai_plugin_transport.go",
    "backend/internal/service/openai_raw_relay_ws.go",
    "backend/internal/service/openai_upstream_transport_error.go",
    "backend/internal/service/openai_ws_forwarder.go",
    "backend/internal/service/openai_ws_forwarder_ingress.go",
    "backend/internal/service/openai_ws_http_bridge.go",
    "backend/internal/service/openai_ws_http_bridge_test.go",
    "backend/internal/service/openai_ws_v2_passthrough_adapter.go",
    "deploy/Dockerfile",
)

# S0–S4 的精确追加兼容合同；正式 merge 必须保留候选完整运行树并包含此锚点。
CUMULATIVE_UPGRADE_ID = "klno.5-tps.1-to-s0-s4"
CUMULATIVE_OLD_REVISION = "9397eb8afb621aef483f2ec0bf4b2dd6247c7b92"
CUMULATIVE_OLD_VERSION = "0.2.14-klno.5-tps.1"
CUMULATIVE_NEW_REVISION = "d72a3dafbdcb1edb642fca8dcfb29a803eaa97af"
CUMULATIVE_OLD_RUNTIME = "928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d"
CUMULATIVE_NEW_RUNTIME = "ecbf079cd17f89f7604987d35a2740ea90264d7327178ed5c7ee2d61e73f9747"
CUMULATIVE_CHANGED_PATHS = (
    "backend/cmd/s44-ws-fixture/driver.go",
    "backend/cmd/s44-ws-fixture/fixture_test.go",
    "backend/cmd/s44-ws-fixture/main.go",
    "backend/cmd/s44-ws-fixture/server.go",
    "backend/cmd/server/wire.go",
    "backend/cmd/server/wire_gen.go",
    "backend/cmd/server/wire_gen_test.go",
    "backend/ent/migrate/schema.go",
    "backend/ent/mutation.go",
    "backend/ent/runtime/runtime.go",
    "backend/ent/schema/usage_log.go",
    "backend/ent/usagelog.go",
    "backend/ent/usagelog/usagelog.go",
    "backend/ent/usagelog/where.go",
    "backend/ent/usagelog_create.go",
    "backend/ent/usagelog_update.go",
    "backend/go.sum",
    "backend/internal/handler/admin/ops_handler.go",
    "backend/internal/handler/admin/ops_routing_diagnostics_handler_test.go",
    "backend/internal/handler/admin/system_handler.go",
    "backend/internal/handler/admin/system_handler_test.go",
    "backend/internal/handler/available_channel_handler.go",
    "backend/internal/handler/available_model_catalog.go",
    "backend/internal/handler/available_model_catalog_test.go",
    "backend/internal/handler/dto/mappers.go",
    "backend/internal/handler/dto/mappers_usage_test.go",
    "backend/internal/handler/dto/types.go",
    "backend/internal/handler/grok_audio.go",
    "backend/internal/handler/handler.go",
    "backend/internal/handler/openai_gateway_credential_failover_loop_test.go",
    "backend/internal/handler/openai_gateway_handler.go",
    "backend/internal/handler/openai_gateway_ws_current_turn_model_test.go",
    "backend/internal/handler/openai_ws_routing_diagnostics.go",
    "backend/internal/handler/ops_error_logger.go",
    "backend/internal/handler/ops_routing_diagnostics_context_test.go",
    "backend/internal/handler/ops_routing_diagnostics_log_test.go",
    "backend/internal/handler/s35_http_routing_behavior_test.go",
    "backend/internal/handler/s35_ws_routing_behavior_test.go",
    "backend/internal/handler/service_status_handler.go",
    "backend/internal/handler/service_status_handler_test.go",
    "backend/internal/handler/service_status_observation_test.go",
    "backend/internal/handler/wire.go",
    "backend/internal/repository/available_catalog_contract_integration_test.go",
    "backend/internal/repository/ops_repo.go",
    "backend/internal/repository/ops_repo_args_test.go",
    "backend/internal/repository/ops_repo_routing_diagnostics_integration_test.go",
    "backend/internal/repository/service_status_incidents.go",
    "backend/internal/repository/service_status_normalize.go",
    "backend/internal/repository/service_status_normalize_test.go",
    "backend/internal/repository/service_status_repo.go",
    "backend/internal/repository/service_status_repo_integration_test.go",
    "backend/internal/repository/service_status_source_integration_test.go",
    "backend/internal/repository/usage_log_repo_insert.go",
    "backend/internal/repository/usage_log_repo_insert_shape_unit_test.go",
    "backend/internal/repository/usage_log_repo_query.go",
    "backend/internal/repository/usage_log_repo_request_type_test.go",
    "backend/internal/repository/usage_log_repo_safety_buffering_unit_test.go",
    "backend/internal/repository/usage_log_session_id_unit_test.go",
    "backend/internal/repository/usage_log_timing_integration_test.go",
    "backend/internal/repository/wire.go",
    "backend/internal/server/api_contract_test.go",
    "backend/internal/server/middleware/middleware.go",
    "backend/internal/server/routes/admin.go",
    "backend/internal/server/routes/service_status_routes_test.go",
    "backend/internal/service/billing_context_schedule.go",
    "backend/internal/service/billing_service.go",
    "backend/internal/service/catalog_pricing.go",
    "backend/internal/service/catalog_pricing_test.go",
    "backend/internal/service/channel_catalog.go",
    "backend/internal/service/gateway_service.go",
    "backend/internal/service/model_pricing_resolver.go",
    "backend/internal/service/openai_account_scheduler.go",
    "backend/internal/service/openai_gateway_forward.go",
    "backend/internal/service/openai_gateway_passthrough.go",
    "backend/internal/service/openai_gateway_passthrough_function_args_test.go",
    "backend/internal/service/openai_gateway_response_handling.go",
    "backend/internal/service/openai_gateway_scheduling.go",
    "backend/internal/service/openai_gateway_service.go",
    "backend/internal/service/openai_gateway_usage.go",
    "backend/internal/service/openai_routing_diagnostics_test.go",
    "backend/internal/service/openai_usage_timing_billing_test.go",
    "backend/internal/service/openai_usage_timing_test.go",
    "backend/internal/service/openai_ws_forwarder_client_cancel_test.go",
    "backend/internal/service/openai_ws_forwarder_ingress.go",
    "backend/internal/service/openai_ws_forwarder_ingress_session_test.go",
    "backend/internal/service/openai_ws_forwarder_success_test.go",
    "backend/internal/service/openai_ws_forwarder_v2.go",
    "backend/internal/service/openai_ws_http_bridge.go",
    "backend/internal/service/openai_ws_native_resume_test.go",
    "backend/internal/service/openai_ws_v2/passthrough_relay.go",
    "backend/internal/service/openai_ws_v2/usage_timing_callback_test.go",
    "backend/internal/service/openai_ws_v2_passthrough_adapter.go",
    "backend/internal/service/ops_models.go",
    "backend/internal/service/ops_port.go",
    "backend/internal/service/ops_routing_diagnostics_dto_test.go",
    "backend/internal/service/ops_routing_diagnostics_log_test.go",
    "backend/internal/service/ops_service.go",
    "backend/internal/service/ops_stream_turn_ownership_test.go",
    "backend/internal/service/ops_upstream_context.go",
    "backend/internal/service/pricing_service.go",
    "backend/internal/service/routing_diagnostics.go",
    "backend/internal/service/routing_diagnostics_test.go",
    "backend/internal/service/routing_diagnostics_validation.go",
    "backend/internal/service/service_status.go",
    "backend/internal/service/service_status_aggregator.go",
    "backend/internal/service/service_status_aggregator_test.go",
    "backend/internal/service/service_status_observation.go",
    "backend/internal/service/service_status_observation_test.go",
    "backend/internal/service/service_status_source_integrity.go",
    "backend/internal/service/service_status_source_integrity_test.go",
    "backend/internal/service/service_status_source_reporter.go",
    "backend/internal/service/service_status_source_reporter_test.go",
    "backend/internal/service/service_status_test.go",
    "backend/internal/service/service_status_types.go",
    "backend/internal/service/stream_output_timing.go",
    "backend/internal/service/stream_output_timing_test.go",
    "backend/internal/service/update_service.go",
    "backend/internal/service/update_service_test.go",
    "backend/internal/service/usage_log.go",
    "backend/internal/service/usage_timing.go",
    "backend/internal/service/wire.go",
    "backend/internal/web/embed_on.go",
    "backend/internal/web/embed_test.go",
    "backend/internal/web/static_cache.go",
    "backend/migrations/251_add_usage_log_timing.sql",
    "backend/migrations/252_add_ops_routing_diagnostics.sql",
    "backend/migrations/253_service_status_observation.sql",
    "backend/migrations/254_service_status.sql",
)
# 与 Go runner 的 SHA256(strings.TrimSpace(SQL)) 相同，不是原文件字节 hash。
CUMULATIVE_MIGRATIONS = {
    "251_add_usage_log_timing.sql": "99562d6463ffa0477e2760c02c3251109e1f7142cf0b86ea4394f871e7a48faf",
    "252_add_ops_routing_diagnostics.sql": "863bd7e74c5fcc5f425e43dcdbfb52691a9fd77e1a00cb2dd8251ea4bde24d0f",
    "253_service_status_observation.sql": "0709bb5a752147c14740d199c9fd84cc9875f3823a79111624a92afe83821c44",
    "254_service_status.sql": "21522e3ef19e18bbe00bef39d48b153b61e0eee3e33b4047b7d045c5edaf0102",
}

# REL-002 固定发布源码的单向基线；须包含此锚点且完整运行树不变。
KIN_UPGRADE_ID = "klno.5-tps.2-to-0.2.15-klno.2"
KIN_OLD_REVISION = "3d5e1fde82707900a21f2b5538b112c7bab3c04a"
KIN_OLD_VERSION = "0.2.14-klno.5-tps.2"
KIN_OLD_IMAGE = REPOSITORY + "@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530"
KIN_NEW_SOURCE = {
    "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.15-klno.2",
    "upstream_sha": "ca686de5e5ac14834e12701c156831abd5ec6b71",
}
KIN_NEW_REVISION = "42907bcab278f93db48d7e1b39b28e28f664569e"
KIN_OLD_RUNTIME = "ecbf079cd17f89f7604987d35a2740ea90264d7327178ed5c7ee2d61e73f9747"
KIN_NEW_RUNTIME = "90e2851678966e6c27e41f5d7ec6e4d0c1196f2a7a8c0c81a1ba84a34c30e7a6"
# 绑定完整排序后的 changed_paths JSON，而非路径范围；证明仍保存全部实际路径。
KIN_CHANGED_PATHS_SHA256 = "e22db48c79378e47c4b30d02541d83476aede91a58886afa6dcf794562e21c24"
KIN_MIGRATIONS = {
    "242_drop_platform_check_constraints.sql": "a7725f31f637208d200e0f37d5270e9f263743d99fa9e4f1bb9a7387e8be5a61",
}
KIN_OLD_MIGRATION_STATE = {
    "rows": 307, "filename_checksum_sha256": "d96e2af9dccac1570317feebfe5fd0c0443340a8e07a722c4a6b9ff22277ef4a",
}

# OBS 已验收应用；后继文档/控制提交须保留此锚点和完整运行树。
OBS_UPGRADE_ID = "429-tps.1-to-obs-tps.2"
OBS_OLD_REVISION = KIN_NEW_REVISION
OBS_OLD_VERSION = "0.2.15-klno.2-tps.1"
OBS_OLD_IMAGE = REPOSITORY + "@sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3"
OBS_NEW_REVISION = "b663258dd6c93a794f2dcfe716979e4dd37eaccd"
OBS_NEW_VERSION = "0.2.15-klno.2-tps.2"
OBS_NEW_RUNTIME = "3d3a9765382ecc98a2a9ad95d6940377b618d1f3e1e5675a16b734b6cfccb9c0"
OBS_OLD_FRONTEND = "ef9884449ee278c275c02b7fd06c34978116b8c3d7c8052694d46d118aa52f4c"
OBS_NEW_FRONTEND = "47b21affa698058f84a71bcce078c4e105965017a9f34c5707886d25a3e0a367"
# 这两份 ?raw 文档从 frontend 外部嵌入；旧429与OBS423字节相同。
OBS_RAW_PATHS = ("docs/legal/admin-compliance.en.md", "docs/legal/admin-compliance.zh.md")
OBS_RAW_SHA256 = "70666fc568db23059802282f36ec4d3fa25a3a2cc0fdc3bbd56c8a24a5808942"
OBS_FRONTEND_CHANGED_PATHS = (
    "frontend/src/api/__tests__/admin.serviceStatus.spec.ts",
    "frontend/src/api/admin/ops.ts",
    "frontend/src/api/admin/serviceStatus.ts",
    "frontend/src/api/client.ts",
    "frontend/src/components/service-status/ServiceStatusMetrics.vue",
    "frontend/src/i18n/locales/en/admin/ops.ts",
    "frontend/src/i18n/locales/en/admin/serviceStatus.ts",
    "frontend/src/i18n/locales/zh/admin/ops.ts",
    "frontend/src/i18n/locales/zh/admin/serviceStatus.ts",
    "frontend/src/views/admin/ServiceStatusView.vue",
    "frontend/src/views/admin/__tests__/ServiceStatusView.spec.ts",
    "frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue",
    "frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts",
)
OBS_CHANGED_PATHS = (
    "backend/internal/handler/admin/ops_handler.go",
    "backend/internal/handler/admin/ops_routing_diagnostics_handler_test.go",
    "backend/internal/handler/service_status_handler.go",
    "backend/internal/repository/service_status_aggregator_monitor_test.go",
    "backend/internal/repository/service_status_contract_integration_test.go",
    "backend/internal/repository/service_status_incidents.go",
    "backend/internal/repository/service_status_monitor.go",
    "backend/internal/repository/service_status_monitor_integration_test.go",
    "backend/internal/repository/service_status_monitor_test.go",
    "backend/internal/repository/service_status_normalize_test.go",
    "backend/internal/repository/service_status_privacy_integration_test.go",
    "backend/internal/repository/service_status_repo.go",
    "backend/internal/repository/service_status_repo_integration_test.go",
    "backend/internal/repository/service_status_watch_integration_test.go",
    "backend/internal/server/middleware/audit_log.go",
    "backend/internal/server/middleware/diagnostics_privacy_test.go",
    "backend/internal/server/middleware/panel_rate_limit.go",
    "backend/internal/server/routes/admin.go",
    "backend/internal/server/routes/diagnostics_privacy_routes_test.go",
    "backend/internal/server/routes/ops_routing_diagnostics_routes_test.go",
    "backend/internal/server/routes/service_status_routes_jwt_test.go",
    "backend/internal/server/routes/service_status_routes_test.go",
    "backend/internal/service/service_status.go",
    "backend/internal/service/service_status_aggregator.go",
    "backend/internal/service/service_status_aggregator_test.go",
    "backend/internal/service/service_status_privacy.go",
    "backend/internal/service/service_status_privacy_test.go",
    "backend/internal/service/service_status_source_integrity.go",
    "backend/internal/service/service_status_source_integrity_test.go",
    "backend/internal/service/service_status_source_reporter.go",
    "backend/internal/service/service_status_test.go",
    "backend/internal/service/service_status_types.go",
)

# RESP 只从实际已发布 C08 升级；同树后继须包含 PR50 和门禁修复后的固定锚点。
RESP_UPGRADE_ID = "obs-tps.2-to-resp-tps.3"
RESP_OLD_REVISION = "c08fba12eb4cd30e65c4177f3acbe20567ecd252"
RESP_OLD_VERSION = OBS_NEW_VERSION
RESP_OLD_IMAGE = REPOSITORY + "@sha256:c4ffab7a920f05564eafe055bdb3171772b145ca5e2ccda690fe8b554bf6fa25"
RESP_NEW_REVISION = "8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90"
RESP_NEW_VERSION = "0.2.15-klno.2-tps.3"
RESP_OLD_RUNTIME = OBS_NEW_RUNTIME
RESP_NEW_RUNTIME = "9f0490b6a346e885dc1e9ff4d9296b15e4f71161a4d9458072cf4ba18f170a48"
RESP_FRONTEND = OBS_NEW_FRONTEND
RESP_RAW_PATHS = OBS_RAW_PATHS
RESP_RAW_SHA256 = OBS_RAW_SHA256
RESP_CHANGED_PATHS = (
    "backend/internal/config/config.go",
    "backend/internal/config/config_test.go",
    "backend/internal/handler/openai_gateway_handler.go",
    "backend/internal/handler/openai_responses_partial_usage_test.go",
    "backend/internal/handler/openai_responses_sticky_admission_test.go",
    "backend/internal/service/gateway_service.go",
    "backend/internal/service/openai_account_scheduler.go",
    "backend/internal/service/openai_forward_partial_usage_test.go",
    "backend/internal/service/openai_gateway_forward.go",
    "backend/internal/service/openai_gateway_passthrough.go",
    "backend/internal/service/openai_gateway_response_handling.go",
    "backend/internal/service/openai_gateway_scheduling.go",
    "backend/internal/service/openai_gateway_service.go",
    "backend/internal/service/openai_gateway_usage.go",
    "backend/internal/service/openai_gateway_usage_integrity.go",
    "backend/internal/service/openai_responses_early_preamble_test.go",
    "backend/internal/service/openai_responses_sticky_spillover_test.go",
    "deploy/.env.example",
    "deploy/config.example.yaml",
    "deploy/docker-compose.dev.yml",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
    "deploy/docker-compose.yml",
)

# .15-klno.2 → .4 仅从生产 RESP 固定发布升级；三项破坏性数据迁移不允许镜像回退。
KLNO4_UPGRADE_ID = "klno.2-tps.3-to-klno.4-tps.1"
KLNO4_OLD_REVISION = RESP_NEW_REVISION
KLNO4_OLD_VERSION = RESP_NEW_VERSION
KLNO4_OLD_IMAGE = REPOSITORY + "@sha256:a216fd8fb138aa1889278df7fbc4a0dd24a09d974886f198cc5102eac6a1b627"
KLNO4_NEW_SOURCE = {
    "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.15-klno.4",
    "upstream_sha": "c555ad918b08cb24ccb2eb45f4ae44a843e24baf",
}
# 正式 merge 必须包含此已提交候选；文档/控制后继不得改变任何应用构建输入。
KLNO4_NEW_REVISION = "045cd71115d48111258bb051d275ca385d4078ed"
KLNO4_NEW_VERSION = "0.2.15-klno.4-tps.1"
KLNO4_NEW_RUNTIME = "102aac962476b496dc29ef33610e31adefb9fb6e60e7065c77e803c6849e71a2"
KLNO4_NEW_FRONTEND = "71c1f6c1b119fa6b4b4f241a7ca8e92be59cf9fbd1c6c38d7d8f121115df27b6"
KLNO4_RAW_PATHS = RESP_RAW_PATHS
KLNO4_RAW_SHA256 = "70666fc568db23059802282f36ec4d3fa25a3a2cc0fdc3bbd56c8a24a5808942"
KLNO4_CHANGED_PATHS_SHA256 = "7129c5c47d76706d4881ca4a214c4164294f554d6161917e91c93595f47d0c15"
KLNO4_FRONTEND_CHANGED_PATHS = (
    "frontend/src/api/__tests__/admin.accounts.gatewayPoolProgress.spec.ts",
    "frontend/src/api/admin/accounts.ts",
    "frontend/src/components/account/AccountCapacityCell.vue",
    "frontend/src/components/account/AccountGatewayCell.vue",
    "frontend/src/components/account/AccountStatusIndicator.vue",
    "frontend/src/components/account/AccountTurnStateCell.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/GatewayPoolBulkSettings.vue",
    "frontend/src/components/account/GatewayUsedHistory.vue",
    "frontend/src/components/account/__tests__/AccountCapacityCell.spec.ts",
    "frontend/src/components/account/__tests__/AccountGatewayCell.runtimeDisplay.spec.ts",
    "frontend/src/components/account/__tests__/AccountGatewayCell.spec.ts",
    "frontend/src/components/account/__tests__/AccountGatewayQueueCards.spec.ts",
    "frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts",
    "frontend/src/components/account/__tests__/AccountTurnStateCell.spec.ts",
    "frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts",
    "frontend/src/components/account/__tests__/EditAccountModal.spec.ts",
    "frontend/src/components/account/__tests__/GatewayUsedHistory.spec.ts",
    "frontend/src/components/account/__tests__/gatewayRuntimeDisplay.fixture.json",
    "frontend/src/components/admin/account/AccountActionMenu.vue",
    "frontend/src/components/admin/account/__tests__/AccountActionMenu.spark_shadow.spec.ts",
    "frontend/src/composables/__tests__/useGatewayPoolProgress.spec.ts",
    "frontend/src/composables/__tests__/useNowTicker.spec.ts",
    "frontend/src/composables/useGatewayPoolProgress.ts",
    "frontend/src/composables/useNowTicker.ts",
    "frontend/src/composables/usePausedDisplay.ts",
    "frontend/src/i18n/__tests__/gwpoolLocales.spec.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/utils/__tests__/gatewayContactLatest.spec.ts",
    "frontend/src/utils/__tests__/gatewayContactStats.spec.ts",
    "frontend/src/utils/__tests__/gatewayUsedSummary.spec.ts",
    "frontend/src/utils/gatewayContactStats.ts",
    "frontend/src/utils/gatewayUsedSummary.ts",
    "frontend/src/utils/turnState.ts",
    "frontend/src/views/admin/AccountsView.vue",
)
KLNO4_MIGRATIONS = {
    "251_drop_gwpool_previous_ledgers.sql": "7d0bbc6f82ce83b9a873a884908cd6a250f4f0a9b6ec991a41201956fbc9d1d1",
    "252_drop_retired_turn_state_and_gwpool_keys.sql": "687f8f1978bd92b75291a56aa2c13c0b2e01b1ad1f4436106211446d65c1512b",
    "253_move_gwpool_ledgers_to_table.sql": "0e84fa98c52d09464d644980ba6f514195c319d4554e5244c11885067d7ffb06",
}
KLNO4_OLD_MIGRATION_STATE = {
    "rows": 308, "filename_checksum_sha256": "a31c942e9e772bef1af9c68f930ee35afaa8c3591b5d1c22609f9bf7d30da882",
}


def approved_klno4_upgrade(direction):
    require(direction in ("upgrade", "redeploy") and SHA_RE.fullmatch(KLNO4_NEW_REVISION) and
            all(re.fullmatch(r"[0-9a-f]{64}", value) for value in
                (KLNO4_NEW_RUNTIME, KLNO4_NEW_FRONTEND, KLNO4_RAW_SHA256, KLNO4_CHANGED_PATHS_SHA256)),
            "E_COMPATIBILITY", ".4 必须绑定最终审定候选；不允许旧镜像回退")
    redeploy = direction == "redeploy"
    return {"id": KLNO4_UPGRADE_ID, "direction": direction,
            "old_source": dict(KLNO4_NEW_SOURCE if redeploy else KIN_NEW_SOURCE),
            "new_source": dict(KLNO4_NEW_SOURCE),
            "old_runtime_revision": KLNO4_NEW_REVISION if redeploy else KLNO4_OLD_REVISION,
            "new_runtime_revision": KLNO4_NEW_REVISION,
            "old_runtime_sha256": KLNO4_NEW_RUNTIME if redeploy else RESP_NEW_RUNTIME,
            "new_runtime_sha256": KLNO4_NEW_RUNTIME,
            "changed_paths_sha256": sha(canonical([])) if redeploy else KLNO4_CHANGED_PATHS_SHA256,
            "frontend_paths": ["frontend"],
            "old_frontend_sha256": KLNO4_NEW_FRONTEND if redeploy else RESP_FRONTEND,
            "new_frontend_sha256": KLNO4_NEW_FRONTEND,
            "frontend_changed_paths": [] if redeploy else list(KLNO4_FRONTEND_CHANGED_PATHS),
            "raw_paths": list(KLNO4_RAW_PATHS), "raw_sha256": KLNO4_RAW_SHA256,
            "image_rollback_compatible": False, "schema_rollback": "retain",
            "migrations": dict(KLNO4_MIGRATIONS)}


def require_klno4_old_image(proof, old):
    if proof.get("klno4_upgrade", {}).get("direction") == "upgrade":
        require(old["revision"] == KLNO4_OLD_REVISION and old["version"] == KLNO4_OLD_VERSION and
                old["image"] == KLNO4_OLD_IMAGE and old["source"] == SOURCE,
                "E_COMPATIBILITY", ".4 必须绑定固定生产 RESP 旧 revision、版本、来源与 digest")


def approved_resp_upgrade(direction):
    require(direction in ("upgrade", "redeploy"), "E_COMPATIBILITY", "RESP 证明不允许旧镜像回退")
    redeploy = direction == "redeploy"
    return {"id": RESP_UPGRADE_ID, "direction": direction,
            "old_source": dict(KIN_NEW_SOURCE), "new_source": dict(KIN_NEW_SOURCE),
            "old_runtime_revision": RESP_NEW_REVISION if redeploy else RESP_OLD_REVISION,
            "new_runtime_revision": RESP_NEW_REVISION,
            "old_runtime_sha256": RESP_NEW_RUNTIME if redeploy else RESP_OLD_RUNTIME,
            "new_runtime_sha256": RESP_NEW_RUNTIME,
            "frontend_paths": ["frontend"], "old_frontend_sha256": RESP_FRONTEND,
            "new_frontend_sha256": RESP_FRONTEND, "frontend_changed_paths": [],
            "raw_paths": list(RESP_RAW_PATHS), "raw_sha256": RESP_RAW_SHA256,
            "image_rollback_compatible": False, "schema_rollback": "retain",
            "migrations": {**CUMULATIVE_MIGRATIONS, **KIN_MIGRATIONS}}


def require_resp_old_image(proof, old):
    if proof.get("resp_upgrade", {}).get("direction") == "upgrade":
        require(old["revision"] == RESP_OLD_REVISION and old["version"] == RESP_OLD_VERSION and
                old["image"] == RESP_OLD_IMAGE and old["source"] == SOURCE,
                "E_COMPATIBILITY", "RESP 必须绑定固定 C08 旧发布 revision、版本、来源与 digest")


def approved_obs_upgrade(direction):
    require(direction in ("upgrade", "redeploy"), "E_COMPATIBILITY", "OBS 证明不允许旧镜像回退")
    redeploy = direction == "redeploy"
    return {"id": OBS_UPGRADE_ID, "direction": direction,
            "old_source": dict(KIN_NEW_SOURCE), "new_source": dict(KIN_NEW_SOURCE),
            "old_runtime_revision": OBS_NEW_REVISION if redeploy else OBS_OLD_REVISION,
            "new_runtime_revision": OBS_NEW_REVISION,
            "old_runtime_sha256": OBS_NEW_RUNTIME if redeploy else KIN_NEW_RUNTIME,
            "new_runtime_sha256": OBS_NEW_RUNTIME,
            "frontend_paths": ["frontend"],
            "old_frontend_sha256": OBS_NEW_FRONTEND if redeploy else OBS_OLD_FRONTEND,
            "new_frontend_sha256": OBS_NEW_FRONTEND,
            "frontend_changed_paths": [] if redeploy else list(OBS_FRONTEND_CHANGED_PATHS),
            "raw_paths": list(OBS_RAW_PATHS), "raw_sha256": OBS_RAW_SHA256,
            "image_rollback_compatible": False, "schema_rollback": "retain",
            "migrations": {**CUMULATIVE_MIGRATIONS, **KIN_MIGRATIONS}}


def require_obs_old_image(proof, old):
    if proof.get("obs_upgrade", {}).get("direction") == "upgrade":
        require(old["image"] == OBS_OLD_IMAGE, "E_COMPATIBILITY", "OBS 必须绑定固定 429 旧发布 digest")


def approved_forward_upgrade():
    return {"id": FORWARD_UPGRADE_ID, "direction": "upgrade",
            "old_source": dict(FORWARD_OLD_SOURCE), "new_source": dict(FORWARD_NEW_SOURCE),
            "old_runtime_revision": FORWARD_OLD_REVISION, "new_runtime_revision": FORWARD_NEW_REVISION,
            "old_runtime_sha256": FORWARD_OLD_RUNTIME, "new_runtime_sha256": FORWARD_NEW_RUNTIME,
            "image_rollback_compatible": False}


def approved_cumulative_upgrade(direction):
    require(SHA_RE.fullmatch(CUMULATIVE_NEW_REVISION) and
            re.fullmatch(r"[0-9a-f]{64}", CUMULATIVE_NEW_RUNTIME) and CUMULATIVE_CHANGED_PATHS,
            "E_COMPATIBILITY", "S0–S4 最终候选尚未固定，禁止部署")
    old_revision, new_revision = CUMULATIVE_OLD_REVISION, CUMULATIVE_NEW_REVISION
    old_runtime, new_runtime = CUMULATIVE_OLD_RUNTIME, CUMULATIVE_NEW_RUNTIME
    if direction == "rollback":
        old_revision, new_revision = new_revision, old_revision
        old_runtime, new_runtime = new_runtime, old_runtime
    return {"id": CUMULATIVE_UPGRADE_ID, "direction": direction,
            "old_source": dict(FORWARD_NEW_SOURCE), "new_source": dict(FORWARD_NEW_SOURCE),
            "old_runtime_revision": old_revision, "new_runtime_revision": new_revision,
            "old_runtime_sha256": old_runtime, "new_runtime_sha256": new_runtime,
            "image_rollback_compatible": True, "schema_rollback": "retain",
            "migrations": dict(CUMULATIVE_MIGRATIONS)}


def approved_kin_upgrade(direction):
    require(direction in ("upgrade", "redeploy"), "E_COMPATIBILITY", "新 KIN 证明不允许旧镜像回退")
    redeploy = direction == "redeploy"
    return {"id": KIN_UPGRADE_ID, "direction": direction,
            "old_source": dict(KIN_NEW_SOURCE if redeploy else FORWARD_NEW_SOURCE),
            "new_source": dict(KIN_NEW_SOURCE),
            "old_runtime_revision": KIN_NEW_REVISION if redeploy else KIN_OLD_REVISION,
            "new_runtime_revision": KIN_NEW_REVISION,
            "old_runtime_sha256": KIN_NEW_RUNTIME if redeploy else KIN_OLD_RUNTIME,
            "new_runtime_sha256": KIN_NEW_RUNTIME,
            "changed_paths_sha256": sha(canonical([])) if redeploy else KIN_CHANGED_PATHS_SHA256,
            "image_rollback_compatible": False, "schema_rollback": "retain",
            "migrations": dict(KIN_MIGRATIONS)}


def validate_compatibility_proof(proof, old_revision, new_revision, old_version=None, new_version=None):
    require(isinstance(proof, dict), "E_COMPATIBILITY", "兼容证据必须为对象")
    content = {key: value for key, value in proof.items() if key != "sha256"}
    require(proof.get("sha256") == sha(canonical(content)) and proof.get("compatible") is True and
            proof.get("old_revision") == old_revision and proof.get("new_revision") == new_revision and
            proof.get("paths") == RUNTIME_PATHS and proof.get("excludes") == RUNTIME_EXCLUDES and
            isinstance(proof.get("changed_paths"), list) and
            all(isinstance(path, str) for path in proof["changed_paths"]),
            "E_COMPATIBILITY", "兼容证据无效或没有绑定实际旧、新 revision")
    forward = proof.get("forward_upgrade")
    cumulative = proof.get("cumulative_upgrade")
    kin = proof.get("kin_upgrade")
    obs = proof.get("obs_upgrade")
    resp = proof.get("resp_upgrade")
    klno4 = proof.get("klno4_upgrade")
    klno4_source = isinstance(proof.get("upstream"), dict) and proof["upstream"].get("upstream_tag") == KLNO4_NEW_SOURCE["upstream_tag"]
    if klno4_source:
        require("klno4_upgrade" in proof, "E_COMPATIBILITY", ".4 必须使用专属完整运行树与破坏性迁移证明")
    if "klno4_upgrade" in proof:
        direction = "upgrade" if old_revision == KLNO4_OLD_REVISION else "redeploy"
        approved = approved_klno4_upgrade(direction)
        require(set(proof) == {"sha256", "old_revision", "new_revision", "upstream", "paths", "excludes",
                               "changed_paths", "compatible", "klno4_upgrade"} and
                isinstance(klno4, dict) and canonical(klno4) == canonical(approved) and
                SHA_RE.fullmatch(old_revision) and SHA_RE.fullmatch(new_revision) and
                proof["upstream"] == KLNO4_NEW_SOURCE and
                sha(canonical(proof["changed_paths"])) == approved["changed_paths_sha256"],
                "E_COMPATIBILITY", ".4 证明不属于固定来源、完整运行树与精确差异")
    kin_source = isinstance(proof.get("upstream"), dict) and proof["upstream"].get("upstream_tag") == KIN_NEW_SOURCE["upstream_tag"]
    if kin_source:
        require("kin_upgrade" in proof or "obs_upgrade" in proof or "resp_upgrade" in proof,
                "E_COMPATIBILITY", "新 KIN 必须使用专属完整运行树与迁移证明")
    if "resp_upgrade" in proof:
        direction = "upgrade" if old_revision == RESP_OLD_REVISION else "redeploy"
        require(set(proof) == {"sha256", "old_revision", "new_revision", "upstream", "paths", "excludes",
                               "changed_paths", "compatible", "resp_upgrade"} and
                isinstance(resp, dict) and canonical(resp) == canonical(approved_resp_upgrade(direction)) and
                SHA_RE.fullmatch(old_revision) and SHA_RE.fullmatch(new_revision) and
                proof["upstream"] == KIN_NEW_SOURCE and
                proof["changed_paths"] == ([] if direction == "redeploy" else list(RESP_CHANGED_PATHS)),
                "E_COMPATIBILITY", "RESP 证明不属于固定来源、完整运行树与精确差异")
    if "obs_upgrade" in proof:
        direction = "upgrade" if old_revision == OBS_OLD_REVISION else "redeploy"
        require(set(proof) == {"sha256", "old_revision", "new_revision", "upstream", "paths", "excludes",
                               "changed_paths", "compatible", "obs_upgrade"} and
                isinstance(obs, dict) and canonical(obs) == canonical(approved_obs_upgrade(direction)) and
                SHA_RE.fullmatch(old_revision) and SHA_RE.fullmatch(new_revision) and
                proof["upstream"] == KIN_NEW_SOURCE and
                proof["changed_paths"] == ([] if direction == "redeploy" else list(OBS_CHANGED_PATHS)),
                "E_COMPATIBILITY", "OBS 证明不属于固定来源、完整运行树与精确差异")
    if "kin_upgrade" in proof:
        direction = "upgrade" if old_revision == KIN_OLD_REVISION else "redeploy"
        approved = approved_kin_upgrade(direction)
        require(set(proof) == {"sha256", "old_revision", "new_revision", "upstream", "paths", "excludes",
                               "changed_paths", "compatible", "kin_upgrade"} and
                isinstance(kin, dict) and canonical(kin) == canonical(approved) and
                SHA_RE.fullmatch(old_revision) and SHA_RE.fullmatch(new_revision) and
                proof["upstream"] == KIN_NEW_SOURCE and
                sha(canonical(proof["changed_paths"])) == approved["changed_paths_sha256"],
                "E_COMPATIBILITY", "新 KIN 证明不属于固定来源、完整运行树与精确差异")
    cumulative_source = isinstance(proof.get("upstream"), dict) and proof["upstream"].get("upstream_tag") == "v0.2.14-klno.5"
    if old_revision != new_revision and CUMULATIVE_OLD_REVISION in (old_revision, new_revision) and cumulative_source and not forward:
        require("cumulative_upgrade" in proof, "E_COMPATIBILITY", "固定 S0–S4 运行差异必须使用专属升级证据")
    if "cumulative_upgrade" in proof:
        direction = "upgrade" if old_revision == CUMULATIVE_OLD_REVISION else "rollback"
        approved = approved_cumulative_upgrade(direction)
        require(set(proof) == {"sha256", "old_revision", "new_revision", "upstream", "paths", "excludes",
                               "changed_paths", "compatible", "cumulative_upgrade"} and
                isinstance(cumulative, dict) and canonical(cumulative) == canonical(approved) and
                (old_revision if direction == "upgrade" else new_revision) == CUMULATIVE_OLD_REVISION and
                old_revision != new_revision and SHA_RE.fullmatch(old_revision) and SHA_RE.fullmatch(new_revision) and
                proof["upstream"] == FORWARD_NEW_SOURCE and proof["changed_paths"] == list(CUMULATIVE_CHANGED_PATHS),
                "E_COMPATIBILITY", "累计升级证据不属于已审定的 revision、来源、运行树与迁移")
    if "forward_upgrade" in proof:
        require(set(proof) == {"sha256", "old_revision", "new_revision", "upstream", "paths", "excludes",
                               "changed_paths", "compatible", "forward_upgrade"} and
                isinstance(forward, dict) and canonical(forward) == canonical(approved_forward_upgrade()) and
                proof["upstream"] == FORWARD_NEW_SOURCE and proof["changed_paths"] == list(FORWARD_CHANGED_PATHS),
                "E_COMPATIBILITY", "正向升级证据不属于已审定的来源、完整运行树与差异")
    if old_version is not None and new_version is not None:
        upgrade = forward or cumulative or kin or obs or resp or klno4
        old_source = upgrade["old_source"] if upgrade else proof.get("upstream", {})
        new_source = upgrade["new_source"] if upgrade else proof.get("upstream", {})
        require(isinstance(old_source, dict) and isinstance(new_source, dict) and
                old_source.get("upstream_tag") == "v" + old_version.split("-tps.", 1)[0] and
                new_source.get("upstream_tag") == "v" + new_version.split("-tps.", 1)[0],
                "E_COMPATIBILITY", "兼容证据的上游来源与实际旧、新镜像版本不一致")
        if cumulative:
            old_release = old_version if cumulative["direction"] == "upgrade" else new_version
            application_release = new_version if cumulative["direction"] == "upgrade" else old_version
            require(old_release == CUMULATIVE_OLD_VERSION and VERSION_RE.fullmatch(application_release) and
                    int(application_release.rsplit("-tps.", 1)[1]) > 1,
                    "E_COMPATIBILITY", "累计升级只绑定固定 .5-tps.1 旧发布与后续同基线个人版本")
        if kin:
            require(VERSION_RE.fullmatch(new_version) and
                    not {OBS_NEW_VERSION, RESP_NEW_VERSION}.intersection((old_version, new_version)) and
                    ((kin["direction"] == "upgrade" and old_version == KIN_OLD_VERSION) or
                     (kin["direction"] == "redeploy" and VERSION_RE.fullmatch(old_version))),
                    "E_COMPATIBILITY", "新 KIN 只绑定固定旧发布与新来源正式个人版本")
        if obs:
            require(new_version == OBS_NEW_VERSION and
                    old_version == (OBS_OLD_VERSION if obs["direction"] == "upgrade" else OBS_NEW_VERSION),
                    "E_COMPATIBILITY", "OBS 只绑定固定 429 旧发布与审定的 tps.2 版本")
        if resp:
            require(new_version == RESP_NEW_VERSION and
                    old_version == (RESP_OLD_VERSION if resp["direction"] == "upgrade" else RESP_NEW_VERSION),
                    "E_COMPATIBILITY", "RESP 只绑定固定 C08 旧发布与审定的 tps.3 版本")
        if klno4:
            require(new_version == KLNO4_NEW_VERSION and
                    old_version == (KLNO4_OLD_VERSION if klno4["direction"] == "upgrade" else KLNO4_NEW_VERSION),
                    "E_COMPATIBILITY", ".4 只绑定生产 RESP 旧发布与审定的正式个人版本")
    if forward or cumulative or kin or obs or resp or klno4:
        return
    if proof["changed_paths"] == []:
        require("security_patch" not in proof, "E_COMPATIBILITY", "无运行差异的证据不能包含安全补丁例外")
        return
    require(proof["changed_paths"] == sorted(SECURITY_PATCH_FILES) and
            proof.get("security_patch") in (approved_security_patch("upgrade"), approved_security_patch("rollback")),
            "E_COMPATIBILITY", "运行差异不属于已审定的精确安全补丁")


def require_image_rollback(proof):
    require(not any(proof.get(name, {}).get("image_rollback_compatible") is False
                    for name in ("forward_upgrade", "cumulative_upgrade", "kin_upgrade", "obs_upgrade", "resp_upgrade", "klno4_upgrade")),
            "E_COMPATIBILITY", "持久化数据回退未获证明；禁止旧镜像恢复，需人工评估失败记录与备份")


def approved_security_patch(direction):
    indexes = (0, 1) if direction == "upgrade" else (1, 0)
    return {"id": SECURITY_PATCH_ID, "direction": direction, "files": {
        path: {"old_sha256": hashes[indexes[0]], "new_sha256": hashes[indexes[1]]}
        for path, hashes in SECURITY_PATCH_FILES.items()}}
