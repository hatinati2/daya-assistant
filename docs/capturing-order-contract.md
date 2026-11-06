# Capturing Daya's order API contract safely

The order API is not documented in the public sources we could access. The Daya login flow is known, but the actual order endpoint and payload must come from the authenticated web application.

## Browser capture

1. Open `https://dayatrader.ir/login` and sign in normally.
2. Open Developer Tools → Network. Enable **Preserve log** and select **Fetch/XHR**.
3. Create one ordinary order through the broker's own interface, using an order you independently intend to place. Do not create a speculative test trade just to generate a request.
4. In Network, locate the request that appears at the same moment as the broker's order submission. Record the request method, endpoint, status, and response.
5. Export the Network log as HAR **with content** to a local file. Do not upload or send the raw HAR.

## Sanitizing locally

From the repository root, run:

```bash
python3 tools/sanitize_daya_har.py /path/to/network.har > daya-order-contract-sanitized.json
```

The script extracts only POST/PUT/PATCH requests to `dayatrader.ir` subdomains. It removes cookie and authorization headers, credentials, CAPTCHA values, account/customer IDs, order IDs, symbols, ISINs, prices, quantities, and query/path identifiers. It preserves endpoint paths, method, content types, JSON field names/types, likely broker enum fields such as `orderSide` and `orderValidityType`, response status, response shape, and short error messages with long numeric/ISIN values redacted.

Run the local test suite with:

```bash
python3 -m unittest discover -s tools -p 'test_*.py'
```

Only share the sanitized JSON output. Inspect it before sharing; if the broker introduces a new sensitive field, remove its value manually. Never share the raw HAR, `Cookie`, `Set-Cookie`, `Authorization`, `token`, username, password, account number, or customer ID.

Once the sanitized request and response are available, the Daya adapter can be implemented and tested with mocked HTTP responses. A live order is not needed for those tests.
