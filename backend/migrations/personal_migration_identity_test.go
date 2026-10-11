package migrations

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// 已在生产和 S44 测试环境执行；这些身份及 checksum 不得随重放重新编号或改写。
func TestMIG001PersonalMigrationIdentity(t *testing.T) {
	checksums := map[string]string{
		"251_add_usage_log_timing.sql":        "99562d6463ffa0477e2760c02c3251109e1f7142cf0b86ea4394f871e7a48faf",
		"252_add_ops_routing_diagnostics.sql": "863bd7e74c5fcc5f425e43dcdbfb52691a9fd77e1a00cb2dd8251ea4bde24d0f",
		"253_service_status_observation.sql":  "0709bb5a752147c14740d199c9fd84cc9875f3823a79111624a92afe83821c44",
		"254_service_status.sql":              "21522e3ef19e18bbe00bef39d48b153b61e0eee3e33b4047b7d045c5edaf0102",
	}
	for name, expected := range checksums {
		t.Run(name, func(t *testing.T) {
			content, err := FS.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			actual := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(string(content)))))
			if actual != expected {
				t.Fatalf("已执行迁移 checksum 改变：得到 %s，要求 %s", actual, expected)
			}
		})
	}
}
