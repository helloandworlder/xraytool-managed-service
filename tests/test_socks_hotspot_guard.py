import importlib.util
import json
import pathlib
import sys
import unittest


MODULE_PATH = pathlib.Path(__file__).resolve().parents[1] / "tools" / "socks_hotspot_guard.py"
spec = importlib.util.spec_from_file_location("socks_hotspot_guard", MODULE_PATH)
guard = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = guard
spec.loader.exec_module(guard)


class SocksHotspotGuardTests(unittest.TestCase):
    def test_parse_ss_counts_groups_by_local_endpoint_and_state(self):
        raw = """State Recv-Q Send-Q Local Address:Port Peer Address:Port Process
ESTAB 0 0 198.63.246.216:7892 45.43.57.72:12345
FIN-WAIT-1 0 0 198.63.246.216:7892 45.43.57.72:12346
FIN-WAIT-1 0 0 198.63.246.216:7892 165.154.134.195:10000
SYN-RECV 0 0 198.63.246.216:7892 1.2.3.4:3333
ESTAB 0 0 198.63.246.79:7892 8.8.8.8:4444
LISTEN 0 8192 198.63.246.216:7892 0.0.0.0:*
"""

        counts = guard.parse_ss_counts(raw)

        self.assertEqual(counts[("198.63.246.216", 7892)]["ESTAB"], 1)
        self.assertEqual(counts[("198.63.246.216", 7892)]["FIN-WAIT-1"], 2)
        self.assertEqual(counts[("198.63.246.216", 7892)]["SYN-RECV"], 1)
        self.assertEqual(counts[("198.63.246.216", 7892)]["LISTEN"], 1)
        self.assertEqual(counts[("198.63.246.79", 7892)]["ESTAB"], 1)

    def test_parse_xray_config_maps_endpoint_accounts(self):
        payload = {
            "inbounds": [
                {
                    "listen": "198.63.246.216",
                    "port": 7892,
                    "protocol": "mixed",
                    "settings": {
                        "accounts": [
                            {"user": "uTFp50q5QOm", "pass": "hidden"},
                            {"user": "other", "pass": "hidden"},
                        ]
                    },
                },
                {"listen": "127.0.0.1", "port": 29585, "protocol": "dokodemo-door"},
            ]
        }

        mapping = guard.parse_xray_config(json.dumps(payload))

        self.assertEqual(mapping[("198.63.246.216", 7892)], ["uTFp50q5QOm", "other"])

    def test_parse_xray_stats_accepts_json_and_calculates_user_totals(self):
        raw = json.dumps(
            {
                "stat": [
                    {"name": "user>>>u1>>>traffic>>>uplink", "value": 100},
                    {"name": "user>>>u1>>>traffic>>>downlink", "value": 500},
                    {"name": "user>>>u2>>>traffic>>>uplink", "value": 7},
                ]
            }
        )

        stats = guard.parse_xray_stats(raw)

        self.assertEqual(stats["u1"].uplink, 100)
        self.assertEqual(stats["u1"].downlink, 500)
        self.assertEqual(stats["u1"].total, 600)
        self.assertEqual(stats["u2"].uplink, 7)
        self.assertEqual(stats["u2"].downlink, 0)

    def test_find_hotspots_filters_by_threshold_and_target(self):
        instance = guard.InstanceSnapshot(
            node="node-a",
            service="xraytool-liunian",
            db_path="/opt/xraytool-liunian/data/xraytool.db",
            api_server="127.0.0.1:29585",
            endpoint_users={("198.63.246.216", 7892): ["uTFp50q5QOm"]},
            connection_counts={
                ("198.63.246.216", 7892): {"ESTAB": 501, "FIN-WAIT-1": 1001},
                ("198.63.246.79", 7892): {"ESTAB": 24},
            },
            user_stats={"uTFp50q5QOm": guard.UserStats(uplink=4096, downlink=0)},
            user_rates={"uTFp50q5QOm": guard.UserRate(up_bps=1024, down_bps=0)},
            order_items={
                "uTFp50q5QOm": guard.OrderItem(
                    order_id=283,
                    order_no="OD260422000283",
                    item_id=283,
                    ip="198.63.246.216",
                    port=7892,
                    username="uTFp50q5QOm",
                    order_status="active",
                    item_status="active",
                    customer="customer-a",
                )
            },
        )

        all_hotspots = guard.find_hotspots([instance], guard.Thresholds(estab=500, fin=1000, speed_bps=0), [])
        targeted = guard.find_hotspots(
            [instance],
            guard.Thresholds(estab=9999, fin=9999, speed_bps=9999),
            [guard.Target(ip="198.63.246.216", port=7892, username="uTFp50q5QOm")],
        )

        self.assertEqual(len(all_hotspots), 1)
        self.assertEqual(all_hotspots[0].username, "uTFp50q5QOm")
        self.assertEqual(all_hotspots[0].order_no, "OD260422000283")
        self.assertEqual(len(targeted), 1)

    def test_build_pause_sql_updates_only_target_order_and_item(self):
        sql = guard.build_pause_sql(283)

        self.assertIn("update orders set status = 'disabled'", sql)
        self.assertIn("where id = 283", sql)
        self.assertIn("update order_items set status = 'disabled'", sql)
        self.assertIn("where order_id = 283", sql)


if __name__ == "__main__":
    unittest.main()
