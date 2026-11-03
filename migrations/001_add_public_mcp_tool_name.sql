-- Separate operator migration. Does not assign names or alter existing gateway behavior.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
ALTER TABLE public.agent_apis ADD COLUMN IF NOT EXISTS public_mcp_tool_name text;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.agent_apis'::regclass
                 AND conname='agent_apis_public_mcp_tool_name_format_check') THEN
    ALTER TABLE public.agent_apis ADD CONSTRAINT agent_apis_public_mcp_tool_name_format_check
      CHECK (public_mcp_tool_name IS NULL OR public_mcp_tool_name ~ '^[a-z][a-z0-9_]{0,63}$');
  END IF;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS agent_apis_public_mcp_tool_name_unique
  ON public.agent_apis (public_mcp_tool_name);
COMMENT ON COLUMN public.agent_apis.public_mcp_tool_name IS
  'Persistent public OpenAny MCP tool name; distinct from upstream mcp_tool_name. NULL awaits assignment.';
COMMIT;
