import json
import unittest

from sanitize_daya_har import extract_entries


class SanitizeDayaHARTests(unittest.TestCase):
    def test_extracts_order_shape_without_secrets_or_trade_values(self):
        har = {
            "log": {
                "entries": [
                    {
                        "request": {
                            "method": "POST",
                            "url": "https://mg.dayatrader.ir/oms/orders?accountId=12345678",
                            "headers": [
                                {"name": "Content-Type", "value": "application/json"},
                                {"name": "Cookie", "value": "cookiesession1=secret-cookie; token=secret-token"},
                                {"name": "Authorization", "value": "Bearer secret-token"},
                                {"name": "x-client-type", "value": "WEB_CLIENT"},
                            ],
                            "postData": {
                                "mimeType": "application/json",
                                "text": json.dumps({
                                    "order": {
                                        "isin": "IRO1TEST0001",
                                        "symbol": "TEST",
                                        "orderSide": 2,
                                        "orderValidityType": 1,
                                        "price": 123456,
                                        "quantity": 987,
                                        "accountId": "secret-account",
                                        "captchaAnswer": "58676",
                                    }
                                }),
                            },
                        },
                        "response": {
                            "status": 200,
                            "headers": [
                                {"name": "Content-Type", "value": "application/json"},
                                {"name": "Set-Cookie", "value": "token=response-secret"},
                            ],
                            "content": {
                                "mimeType": "application/json",
                                "text": json.dumps({
                                    "success": True,
                                    "data": {"orderId": "1234567890123"},
                                    "message": "accepted",
                                }),
                            },
                        },
                    },
                    {
                        "request": {
                            "method": "POST",
                            "url": "https://example.com/not-daya",
                            "postData": {"mimeType": "application/json", "text": "{}"},
                        },
                        "response": {"status": 200, "content": {"text": "{}"}},
                    },
                ]
            }
        }

        entries = extract_entries(har)
        self.assertEqual(len(entries), 1)
        item = entries[0]
        self.assertEqual(item["method"], "POST")
        self.assertEqual(item["request_headers"]["x-client-type"], "WEB_CLIENT")
        self.assertNotIn("cookie", item["request_headers"])
        self.assertNotIn("authorization", item["request_headers"])
        self.assertIn("accountId=%3Credacted%3E", item["url"])
        self.assertEqual(item["request_payload"]["order"]["orderSide"], 2)
        self.assertEqual(item["request_payload"]["order"]["orderValidityType"], 1)
        self.assertEqual(item["request_payload"]["order"]["price"], "<number>")
        self.assertEqual(item["request_payload"]["order"]["quantity"], "<number>")
        self.assertEqual(item["request_payload"]["order"]["isin"], "<ISIN>")
        self.assertEqual(item["response_payload"]["data"]["orderId"], "<redacted>")
        serialized = json.dumps(item)
        for secret in ("secret-cookie", "secret-token", "secret-account", "58676", "123456"):
            self.assertNotIn(secret, serialized)

    def test_ignores_non_mutating_and_non_dayatrader_requests(self):
        har = {
            "log": {
                "entries": [
                    {"request": {"method": "GET", "url": "https://mg.dayatrader.ir/captcha"}},
                    {"request": {"method": "POST", "url": "https://example.org/order"}},
                    {"request": {"method": "DELETE", "url": "https://mg.dayatrader.ir/orders/123"}},
                ]
            }
        }
        self.assertEqual(extract_entries(har), [])

    def test_redacts_ids_in_paths_and_preserves_error_message(self):
        har = {
            "log": {
                "entries": [{
                    "request": {
                        "method": "POST",
                        "url": "https://mg.dayatrader.ir/accounts/123456/orders",
                        "postData": {
                            "mimeType": "application/json",
                            "text": '{"orderSide":1,"quantity":2}',
                        },
                    },
                    "response": {
                        "status": 400,
                        "content": {
                            "text": '{"message":"Rejected for ISIN IRO1TEST0001, reference 123456789"}'
                        },
                    },
                }]
            }
        }
        item = extract_entries(har)[0]
        self.assertIn("/accounts/<id>/orders", item["url"])
        self.assertEqual(
            item["response_payload"]["message"],
            "Rejected for ISIN <ISIN>, reference <digits>",
        )


if __name__ == "__main__":
    unittest.main()
