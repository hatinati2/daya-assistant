import unittest

from sanitize_daya_financial_har import extract_entries


class FinancialHarSanitizerTests(unittest.TestCase):
    def test_extracts_only_daya_get_schema_without_values(self):
        har = {
            "log": {
                "entries": [
                    {
                        "request": {
                            "method": "GET",
                            "url": "https://mg.dayatrader.ir/financials/portfolio/instant?account=123456789",
                        },
                        "response": {
                            "status": 200,
                            "content": {
                                "text": (
                                    '{"content":[{"cashAvailable":"6000000",'
                                    '"availableToWithdraw":4500000,'
                                    '"accountNumber":"123456789",'
                                    '"customerName":"Sensitive Name",'
                                    '"id":"abc"}],"totalElements":1}'
                                )
                            },
                        },
                    },
                    {
                        "request": {
                            "method": "POST",
                            "url": "https://mg.dayatrader.ir/orders",
                        },
                        "response": {
                            "status": 200,
                            "content": {"text": '{"orderStatus":"BOOKED"}'},
                        },
                    },
                    {
                        "request": {
                            "method": "GET",
                            "url": "https://example.com/financials",
                        },
                        "response": {
                            "status": 200,
                            "content": {"text": '{"cashBalance":100}'},
                        },
                    },
                ]
            }
        }
        entries = extract_entries(har)
        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0]["path"], "/financials/portfolio/instant")
        self.assertEqual(entries[0]["status"], 200)
        schema = "\n".join(entries[0]["schema"])
        self.assertIn("content:array", schema)
        self.assertIn("content[].cashAvailable:string", schema)
        self.assertIn("content[].availableToWithdraw:number", schema)
        self.assertIn("totalElements:number", schema)
        for forbidden in ("6000000", "4500000", "123456789", "Sensitive Name", "accountNumber", "customerName", "id"):
            self.assertNotIn(forbidden, schema)


if __name__ == "__main__":
    unittest.main()
