"""Mocked Streamable HTTP coverage for catalog calls, run_api, and failures."""

from __future__ import annotations

import json
import os
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from unittest import mock
ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

import mcp_client
from mcp_client import HttpResponse, OpenAnyCallError, TransportFailure
from provider.openany import OpenAnyProvider
from tools.get_api import GetApiTool
from tools.openany_overview import OpenAnyOverviewTool
from tools.run_api import RunApiTool
from tools.search_apis import SearchApisTool

CANARY = "oa-test-key-do-not-leak-9f3c"
API_VERSION = "a" * 64


def json_response(payload: dict, status: int = 200, headers: dict | None = None) -> HttpResponse:
    response_headers = {"content-type": "application/json"}
    if headers:
        response_headers.update(headers)
    return HttpResponse(status, response_headers, json.dumps(payload).encode())


def initialize_response(session: str | None = None) -> HttpResponse:
    headers = {"mcp-session-id": session} if session else None
    return json_response(
        {
            "jsonrpc": "2.0",
            "id": 1,
            "result": {
                "protocolVersion": "2025-11-25",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "OpenAny", "version": "0.1.0"},
            },
        },
        headers=headers,
    )


def tool_response(result: dict, *, request_id: int = 2) -> HttpResponse:
    return json_response({"jsonrpc": "2.0", "id": request_id, "result": result})


class Script:
    def __init__(self, responses: dict):
        self.responses = responses
        self.calls: list[dict] = []

    def __call__(self, payload, headers, timeout):
        self.calls.append(
            {"payload": payload, "headers": dict(headers), "timeout": timeout}
        )
        item = self.responses[payload["method"]]
        if isinstance(item, Exception):
            raise item
        if callable(item):
            return item(payload, headers)
        return item


class ClientTests(unittest.TestCase):
    def test_overview_is_unauthenticated_and_returns_the_catalog_object(self):
        script = Script(
            {
                "initialize": initialize_response(),
                "notifications/initialized": HttpResponse(202, {}, b""),
                "tools/call": tool_response(
                    {
                        "isError": False,
                        "content": [{"type": "text", "text": json.dumps({"api_count": 3})}],
                        "structuredContent": {"api_count": 3, "model_count": 1},
                    }
                ),
            }
        )
        with mock.patch.object(mcp_client, "exchange", script):
            result = mcp_client.call_tool("openany_overview", {"ignored": "x"})
        self.assertEqual(result, {"api_count": 3, "model_count": 1})
        self.assertEqual(
            [call["payload"]["method"] for call in script.calls],
            ["initialize", "notifications/initialized", "tools/call"],
        )
        self.assertEqual(script.calls[2]["payload"]["params"]["arguments"], {})
        for call in script.calls:
            self.assertNotIn("Authorization", call["headers"])
        self.assertEqual(script.calls[0]["payload"]["params"]["protocolVersion"], "2025-11-25")
        self.assertEqual(script.calls[0]["headers"]["Accept"], "application/json, text/event-stream")

    def test_search_and_get_send_schema_arguments_without_a_key(self):
        script = Script(
            {
                "initialize": initialize_response("sess-1"),
                "notifications/initialized": HttpResponse(202, {}, b""),
                "tools/call": tool_response(
                    {"content": [{"type": "text", "text": "{\"results\":[]}"}]}
                ),
            }
        )
        with mock.patch.object(mcp_client, "exchange", script):
            mcp_client.call_tool(
                "search_apis",
                {
                    "query": " backlinks ",
                    "kind": "api",
                    "provider": "",
                    "limit": 5.0,
                    "cursor": None,
                },
                api_key=CANARY,
            )
        arguments = script.calls[2]["payload"]["params"]["arguments"]
        self.assertEqual(arguments, {"query": "backlinks", "kind": "api", "limit": 5})
        self.assertEqual(script.calls[1]["headers"]["Mcp-Session-Id"], "sess-1")
        self.assertEqual(script.calls[2]["headers"]["Mcp-Session-Id"], "sess-1")
        for call in script.calls:
            self.assertNotIn("Authorization", call["headers"])
            self.assertNotIn(CANARY, json.dumps(call))

    def test_get_api_argument(self):
        script = self._success_script()
        with mock.patch.object(mcp_client, "exchange", script):
            mcp_client.call_tool("get_api", {"api": "demo_api"})
        self.assertEqual(
            script.calls[2]["payload"]["params"],
            {"name": "get_api", "arguments": {"api": "demo_api"}},
        )

    def test_run_api_sends_the_key_only_on_tools_call(self):
        script = self._success_script()
        output = {
            "mode": "compact",
            "items_path": "/rows",
            "fields": ["id", "name"],
            "max_items": 5,
        }
        with mock.patch.object(mcp_client, "exchange", script):
            result = mcp_client.call_tool(
                "run_api",
                {
                    "api": "demo_api",
                    "api_version": API_VERSION,
                    "input": json.dumps({"q": "bitcoin"}),
                    "output": output,
                },
                paid=True,
                api_key="Bearer " + CANARY,
            )
        self.assertEqual(result, {"ok": True})
        self.assertNotIn("Authorization", script.calls[0]["headers"])
        self.assertNotIn("Authorization", script.calls[1]["headers"])
        self.assertEqual(script.calls[2]["headers"]["Authorization"], "Bearer " + CANARY)
        self.assertEqual(script.calls[2]["timeout"], mcp_client.RUN_TIMEOUT)
        arguments = script.calls[2]["payload"]["params"]["arguments"]
        self.assertEqual(arguments["input"], {"q": "bitcoin"})
        self.assertEqual(arguments["output"], output)
        self.assertEqual(len(script.calls), 3)

    def test_serverless_deadline_leaves_room_to_report_a_timeout(self):
        budget = 2 * mcp_client.CATALOG_TIMEOUT + mcp_client.RUN_TIMEOUT
        self.assertEqual(mcp_client.CATALOG_TIMEOUT, 30)
        self.assertEqual(mcp_client.RUN_TIMEOUT, 120)
        self.assertEqual(budget, 180)
        self.assertGreaterEqual(mcp_client.PLUGIN_REQUEST_DEADLINE, 210)
        self.assertGreater(mcp_client.PLUGIN_REQUEST_DEADLINE, budget)
        self.assertEqual(
            mcp_client.PLUGIN_REQUEST_DEADLINE,
            int(budget + mcp_client.RESULT_DELIVERY_MARGIN_SECONDS),
        )
        main = (ROOT / "main.py").read_text()
        self.assertIn("PLUGIN_REQUEST_DEADLINE", main)
        self.assertNotIn("MAX_REQUEST_TIMEOUT=180", main)

    def test_short_secret_is_rejected_before_any_request(self):
        short = "abc1234"

        def explode(*_args, **_kwargs):
            raise AssertionError("network was called")

        with mock.patch.object(mcp_client, "exchange", explode):
            with self.assertRaises(OpenAnyCallError) as caught:
                mcp_client.call_tool(
                    "run_api",
                    {"api": "demo_api", "api_version": API_VERSION, "input": {}},
                    paid=True,
                    api_key=short,
                )
            status, reason = mcp_client.credential_status(short)
            empty_status, _empty_reason = mcp_client.credential_status("")
            saved = OpenAnyProvider().validate_credentials({"openany_api_key": short})
        self.assertEqual(caught.exception.code, "openany_dify_invalid_credentials")
        self.assertEqual(caught.exception.execution_state, "not_started")
        self.assertNotIn(short, str(caught.exception))
        self.assertEqual(status, "invalid")
        self.assertEqual(reason, "credential_invalid")
        self.assertEqual(empty_status, "valid")
        self.assertEqual(saved.status, "invalid")
        self.assertNotIn(short, str(saved.reason_code))

    def test_http_error_excerpt_drops_the_session_and_the_key(self):
        session = "sess-9f3c-do-not-leak"
        body = f"boom Mcp-Session-Id: {session} leaked {CANARY} and again {session}".encode()
        script = self._success_script(
            HttpResponse(500, {"content-type": "text/plain", "mcp-session-id": session}, body)
        )
        script.responses["initialize"] = initialize_response(session)
        with mock.patch.object(mcp_client, "exchange", script):
            with self.assertRaises(OpenAnyCallError) as caught:
                mcp_client.call_tool(
                    "run_api",
                    {"api": "demo_api", "api_version": API_VERSION, "input": {}},
                    paid=True,
                    api_key=CANARY,
                )
        text = str(caught.exception)
        self.assertIn("HTTP 500", text)
        self.assertIn("[REDACTED]", text)
        self.assertNotIn(CANARY, text)
        self.assertNotIn(session, text)

    def test_missing_and_placeholder_keys_do_not_call_the_network(self):
        def explode(*_args, **_kwargs):
            raise AssertionError("network was called")

        with mock.patch.object(mcp_client, "exchange", explode):
            for key in (None, "", "   ", "${OPENANY_API_KEY}", "$OPENANY_API_KEY", "<key>", "YOUR_API_KEY"):
                with self.subTest(key=key):
                    with self.assertRaises(OpenAnyCallError) as caught:
                        mcp_client.call_tool(
                            "run_api",
                            {"api": "demo_api", "api_version": API_VERSION, "input": {}},
                            paid=True,
                            api_key=key,
                        )
                    self.assertEqual(caught.exception.execution_state, "not_started")
                    self.assertNotIn(CANARY, str(caught.exception))
                    if key in (None, "", "   "):
                        self.assertEqual(caught.exception.code, "openany_mcp_credentials_missing")
                    else:
                        self.assertEqual(
                            caught.exception.code,
                            "openany_mcp_credentials_placeholder",
                        )

    def test_invalid_arguments_do_not_call_the_network(self):
        def explode(*_args, **_kwargs):
            raise AssertionError("network was called")

        cases = [
            ("run_api", {"api": "demo_api", "api_version": "ABC", "input": {}}, "openany_mcp_invalid_input"),
            ("run_api", {"api": "demo_api", "api_version": API_VERSION, "input": []}, "openany_mcp_invalid_input"),
            (
                "run_api",
                {
                    "api": "demo_api",
                    "api_version": API_VERSION,
                    "input": {},
                    "output": {"mode": "raw", "fields": ["id"]},
                },
                "openany_mcp_invalid_output",
            ),
            ("search_apis", {"limit": 5}, "openany_mcp_invalid_input"),
            ("get_api", {}, "openany_mcp_invalid_input"),
        ]
        with mock.patch.object(mcp_client, "exchange", explode):
            for name, parameters, code in cases:
                with self.subTest(name=name, parameters=parameters):
                    with self.assertRaises(OpenAnyCallError) as caught:
                        mcp_client.call_tool(
                            name,
                            parameters,
                            paid=name == "run_api",
                            api_key=CANARY,
                        )
                    self.assertEqual(caught.exception.code, code)
                    self.assertEqual(caught.exception.execution_state, "not_started")

    def test_tool_error_is_not_a_success_and_redacts_the_key(self):
        body = {
            "error": {
                "code": "openany_key_invalid",
                "message": f"rejected {CANARY}",
                "execution_state": "reported_failure",
                "next_action": "Replace the key in the provider secret.",
            }
        }
        script = self._success_script(
            tool_response(
                {
                    "isError": True,
                    "content": [{"type": "text", "text": json.dumps(body)}],
                }
            )
        )
        with mock.patch.object(mcp_client, "exchange", script):
            with self.assertRaises(OpenAnyCallError) as caught:
                mcp_client.call_tool(
                    "run_api",
                    {"api": "demo_api", "api_version": API_VERSION, "input": {}},
                    paid=True,
                    api_key=CANARY,
                )
        self.assertEqual(caught.exception.code, "openany_key_invalid")
        self.assertEqual(caught.exception.execution_state, "reported_failure")
        self.assertIn("[REDACTED]", str(caught.exception))
        self.assertNotIn(CANARY, str(caught.exception))
        self.assertEqual(len(script.calls), 3)

    def test_non_json_http_rpc_error_and_empty_result_fail(self):
        cases = {
            "html": HttpResponse(200, {"content-type": "text/html"}, b"<html>nope</html>"),
            "rpc": json_response(
                {"jsonrpc": "2.0", "id": 2, "error": {"code": -32602, "message": "bad params"}}
            ),
            "empty": tool_response({"isError": False, "content": []}),
            "status": HttpResponse(500, {"content-type": "text/plain"}, b"boom"),
        }
        for label, response in cases.items():
            script = self._success_script(response)
            with self.subTest(label=label):
                with mock.patch.object(mcp_client, "exchange", script):
                    with self.assertRaises(OpenAnyCallError) as caught:
                        mcp_client.call_tool("openany_overview", {})
                self.assertNotEqual(caught.exception.execution_state, "")
                self.assertIn("code=", str(caught.exception))

    def test_sse_and_plain_text_results(self):
        sse = (
            "event: message\n"
            "data: " + json.dumps(
                {
                    "jsonrpc": "2.0",
                    "id": 2,
                    "result": {
                        "isError": False,
                        "content": [{"type": "text", "text": "plain catalog note"}],
                    },
                }
            )
            + "\n\n"
        )
        script = self._success_script(
            HttpResponse(200, {"content-type": "text/event-stream"}, sse.encode())
        )
        with mock.patch.object(mcp_client, "exchange", script):
            result = mcp_client.call_tool("openany_overview", {})
        self.assertEqual(result, {"result_kind": "text", "text": "plain catalog note"})

    def test_run_timeout_is_unknown_and_is_not_retried(self):
        def tools_call(_payload, _headers):
            raise TransportFailure("timeout", "request timed out")

        script = Script(
            {
                "initialize": initialize_response(),
                "notifications/initialized": HttpResponse(202, {}, b""),
                "tools/call": tools_call,
            }
        )
        with mock.patch.object(mcp_client, "exchange", script):
            with self.assertRaises(OpenAnyCallError) as caught:
                mcp_client.call_tool(
                    "run_api",
                    {"api": "demo_api", "api_version": API_VERSION, "input": {"q": 1}},
                    paid=True,
                    api_key=CANARY,
                )
        self.assertEqual(caught.exception.code, "openany_dify_timeout")
        self.assertEqual(caught.exception.execution_state, "unknown")
        self.assertIn("Do not retry", str(caught.exception))
        self.assertEqual([call["payload"]["method"] for call in script.calls], [
            "initialize",
            "notifications/initialized",
            "tools/call",
        ])
        self.assertNotIn(CANARY, str(caught.exception))

    def test_initialize_timeout_does_not_send_run_api(self):
        script = Script({"initialize": TransportFailure("timeout", "request timed out")})
        with mock.patch.object(mcp_client, "exchange", script):
            with self.assertRaises(OpenAnyCallError) as caught:
                mcp_client.call_tool(
                    "run_api",
                    {"api": "demo_api", "api_version": API_VERSION, "input": {}},
                    paid=True,
                    api_key=CANARY,
                )
        self.assertEqual(caught.exception.execution_state, "not_started")
        self.assertEqual(len(script.calls), 1)

    def test_redirect_response_is_not_followed_by_another_request(self):
        script = Script(
            {
                "initialize": initialize_response(),
                "notifications/initialized": HttpResponse(202, {}, b""),
                "tools/call": HttpResponse(302, {"location": "https://evil.example/steal"}, b""),
            }
        )
        with mock.patch.object(mcp_client, "exchange", script):
            with self.assertRaises(OpenAnyCallError) as caught:
                mcp_client.call_tool("openany_overview", {})
        self.assertEqual(caught.exception.code, "openany_dify_http_error")
        self.assertEqual(len(script.calls), 3)
        self.assertNotIn("evil.example", str(caught.exception))

    def test_success_payload_redacts_the_key(self):
        script = self._success_script(
            tool_response(
                {"content": [{"type": "text", "text": json.dumps({"echo": CANARY})}]}
            )
        )
        with mock.patch.object(mcp_client, "exchange", script):
            result = mcp_client.call_tool(
                "run_api",
                {"api": "demo_api", "api_version": API_VERSION, "input": {}},
                paid=True,
                api_key=CANARY,
            )
        self.assertEqual(result, {"echo": "[REDACTED]"})
        self.assertNotIn(CANARY, json.dumps(result))

    def test_exchange_refuses_any_other_endpoint(self):
        for url in (
            "http://mcp.openany.si",
            "https://mcp.openany.si/mcp",
            "https://evil.example",
            "https://user:pw@mcp.openany.si",
            "https://mcp.openany.si:8443",
        ):
            with self.subTest(url=url):
                with self.assertRaises(TransportFailure):
                    mcp_client._assert_fixed_endpoint(url)
        seen = []

        def fake_open(url, payload, headers, timeout):
            seen.append(url)
            return HttpResponse(200, {"content-type": "application/json"}, b"{}")

        with mock.patch.object(mcp_client, "open_http", fake_open):
            mcp_client.exchange({"jsonrpc": "2.0", "id": 1}, {}, 1)
        self.assertEqual(seen, ["https://mcp.openany.si"])

    def test_open_http_does_not_follow_redirects_or_proxies(self):
        stolen = []

        class StealHandler(BaseHTTPRequestHandler):
            def do_POST(self):
                stolen.append(self.headers.get("Authorization"))
                self.send_response(200)
                self.end_headers()

            def do_GET(self):
                stolen.append(self.headers.get("Authorization"))
                self.send_response(200)
                self.end_headers()

            def log_message(self, fmt, *args):
                return

        class RedirectHandler(BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers.get("Content-Length", "0"))
                self.rfile.read(length)
                self.server.seen = self.headers.get("Authorization")
                self.send_response(302)
                self.send_header("Location", self.server.location)
                self.end_headers()

            def log_message(self, fmt, *args):
                return

        steal = ThreadingHTTPServer(("127.0.0.1", 0), StealHandler)
        redirect = ThreadingHTTPServer(("127.0.0.1", 0), RedirectHandler)
        redirect.location = f"http://127.0.0.1:{steal.server_address[1]}/steal"
        threads = [
            threading.Thread(target=steal.serve_forever, daemon=True),
            threading.Thread(target=redirect.serve_forever, daemon=True),
        ]
        for thread in threads:
            thread.start()
        previous = {name: os.environ.get(name) for name in ("HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy")}
        proxy = f"http://127.0.0.1:{steal.server_address[1]}"
        os.environ["HTTP_PROXY"] = proxy
        os.environ["HTTPS_PROXY"] = proxy
        os.environ["http_proxy"] = proxy
        os.environ["https_proxy"] = proxy
        target = None
        try:
            with self.assertRaises(TransportFailure) as caught:
                mcp_client.open_http(
                    f"http://127.0.0.1:{redirect.server_address[1]}/",
                    b"{}",
                    {"Authorization": "Bearer " + CANARY, "Content-Type": "application/json"},
                    5,
                )
            self.assertEqual(caught.exception.kind, "redirect")
            self.assertEqual(stolen, [])
            self.assertEqual(redirect.seen, "Bearer " + CANARY)

            class OkHandler(BaseHTTPRequestHandler):
                def do_POST(self):
                    self.server.hits += 1
                    length = int(self.headers.get("Content-Length", "0"))
                    self.rfile.read(length)
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(b"{}")

                def log_message(self, fmt, *args):
                    return

            target = ThreadingHTTPServer(("127.0.0.1", 0), OkHandler)
            target.hits = 0
            threading.Thread(target=target.serve_forever, daemon=True).start()
            try:
                response = mcp_client.open_http(
                    f"http://127.0.0.1:{target.server_address[1]}/",
                    b"{}",
                    {"Content-Type": "application/json"},
                    5,
                )
                self.assertEqual(response.status, 200)
                self.assertEqual(target.hits, 1)
                self.assertEqual(stolen, [])
            finally:
                target.shutdown()
                target.server_close()
        finally:
            redirect.shutdown()
            redirect.server_close()
            steal.shutdown()
            steal.server_close()
            for name, value in previous.items():
                if value is None:
                    os.environ.pop(name, None)
                else:
                    os.environ[name] = value

    def test_provider_validation_is_local(self):
        provider = OpenAnyProvider()

        def explode(*_args, **_kwargs):
            raise AssertionError("validation called the network")

        with mock.patch.object(mcp_client, "exchange", explode):
            empty = provider.validate_credentials({})
            blank = provider.validate_credentials({"openany_api_key": ""})
            saved = provider.validate_credentials({"openany_api_key": CANARY})
            bad = provider.validate_credentials({"openany_api_key": "${OPENANY_API_KEY}"})
        self.assertEqual(empty.status, "valid")
        self.assertEqual(blank.status, "valid")
        self.assertEqual(saved.status, "valid")
        self.assertEqual(bad.status, "invalid")
        self.assertEqual(bad.reason_code, "credential_invalid")
        self.assertNotIn(CANARY, str(bad))
        self.assertNotIn("${OPENANY_API_KEY}", str(bad.reason_code))

    def test_tool_wrappers_raise_instead_of_yielding_success(self):
        script = self._success_script(
            tool_response({"content": [{"type": "text", "text": "{\"api_count\":4}"}]})
        )
        with mock.patch.object(mcp_client, "exchange", script):
            messages = list(OpenAnyOverviewTool.from_credentials({CANARY: "ignored"})._invoke({}))
        self.assertEqual(messages[0].message.json_object["api_count"], 4)
        self.assertIn("api_count", messages[1].message.text)
        self.assertNotIn(CANARY, messages[1].message.text)

        def explode(*_args, **_kwargs):
            raise AssertionError("run_api called the network")

        with mock.patch.object(mcp_client, "exchange", explode):
            with self.assertRaises(OpenAnyCallError):
                list(
                    RunApiTool.from_credentials({})._invoke(
                        {"api": "demo_api", "api_version": API_VERSION, "input": {}}
                    )
                )

        search = Script(
            {
                "initialize": initialize_response(),
                "notifications/initialized": HttpResponse(202, {}, b""),
                "tools/call": tool_response(
                    {"content": [{"type": "text", "text": "{\"results\":[{\"api\":\"demo_api\"}]}"}]}
                ),
            }
        )
        with mock.patch.object(mcp_client, "exchange", search):
            found = list(SearchApisTool.from_credentials({})._invoke({"query": "bitcoin", "limit": "5"}))
        self.assertEqual(found[0].message.json_object["results"][0]["api"], "demo_api")
        self.assertEqual(search.calls[2]["payload"]["params"]["arguments"]["limit"], 5)

        detail = self._success_script(
            tool_response({"structuredContent": {"api": "demo_api", "pricing": {"status": "available"}}})
        )
        with mock.patch.object(mcp_client, "exchange", detail):
            got = list(GetApiTool.from_credentials({})._invoke({"api": "demo_api"}))
        self.assertEqual(got[0].message.json_object["pricing"]["status"], "available")

    def _success_script(self, tools_call: HttpResponse | None = None) -> Script:
        return Script(
            {
                "initialize": initialize_response(),
                "notifications/initialized": HttpResponse(202, {}, b""),
                "tools/call": tools_call
                or tool_response({"content": [{"type": "text", "text": "{\"ok\":true}"}]}),
            }
        )


class PackageSourceTests(unittest.TestCase):
    def test_runtime_files_do_not_embed_a_key_or_the_advanced_catalog_path(self):
        banned = ("sk-", "Bearer oa-", "OPENANY_API_KEY=")
        for path in ROOT.rglob("*"):
            if not path.is_file() or path.suffix not in {".py", ".yaml", ".yml", ".md", ".txt", ".toml"}:
                continue
            if "tests" in path.parts:
                continue
            text = path.read_text()
            for token in banned:
                self.assertNotIn(token, text, path.name)
            if path.suffix in {".py", ".yaml", ".yml"}:
                self.assertNotIn("https://mcp.openany.si/mcp", text, path.name)
                self.assertNotIn("/mcp-lite", text, path.name)


if __name__ == "__main__":
    unittest.main()
