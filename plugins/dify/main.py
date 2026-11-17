from dify_plugin import DifyPluginEnv, Plugin

from mcp_client import PLUGIN_REQUEST_DEADLINE

# Serverless streams stay quiet until the tool returns. This ceiling is the
# SDK idle limit, not the OpenAny HTTP timeout.
plugin = Plugin(DifyPluginEnv(MAX_REQUEST_TIMEOUT=PLUGIN_REQUEST_DEADLINE))

if __name__ == "__main__":
    plugin.run()
