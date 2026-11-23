const URL = "https://mcp.openany.si";

export function openanyServerConfig(env = process.env) {
  const config = {
    url: URL,
    type: "streamable-http",
    exposure: "direct",
    description:
      "OpenAny lite MCP: openany_overview, search_apis, get_api, and run_api. Browse free; run_api is pay per call.",
  };
  const key = env.OPENANY_API_KEY;
  if (typeof key === "string" && key.trim() !== "") {
    config.headers = { Authorization: `Bearer ${key.trim()}` };
  }
  return config;
}

export default function (pi) {
  pi.registerMcpServer("openany", openanyServerConfig());
}
