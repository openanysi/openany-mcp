import os
import threading
import unittest
from pathlib import Path
from sync_mcp_tool_names import synchronize

@unittest.skipUnless(os.getenv('MCP_TEST_DATABASE_URL'), 'requires disposable PostgreSQL URL')
class DatabaseTests(unittest.TestCase):
    def setUp(self):
        import psycopg
        self.url=os.environ['MCP_TEST_DATABASE_URL']
        self.conn=psycopg.connect(self.url,autocommit=True)
        if not self.conn.info.dbname.startswith('openany_mcp_test'):
            self.conn.close()
            raise RuntimeError('Integration database name must start with openany_mcp_test; refusing destructive setup')
        self.conn.execute('DROP TABLE IF EXISTS public.agent_apis CASCADE')
        self.conn.execute('''CREATE TABLE public.agent_apis(id uuid PRIMARY KEY,provider_id text NOT NULL,
          method text NOT NULL,path_template text NOT NULL,status text NOT NULL,updated_at timestamptz DEFAULT now(),price bigint DEFAULT 7)''')
        sql=Path(__file__).resolve().parents[1].joinpath('migrations/001_add_public_mcp_tool_name.sql').read_text()
        self.conn.execute(sql)
        self.conn.execute(sql)
        self.ids=['00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000002']
        for identity in self.ids:self.conn.execute("INSERT INTO public.agent_apis(id,provider_id,method,path_template,status) VALUES(%s,'demo','GET','/list','active')",(identity,))
    def tearDown(self):self.conn.close()
    def test_preview_then_apply_then_noop(self):
        self.assertEqual(synchronize(self.url)['assigned'],2)
        self.assertEqual(self.conn.execute('SELECT count(*) FROM public.agent_apis WHERE public_mcp_tool_name IS NOT NULL').fetchone()[0],0)
        self.assertEqual(synchronize(self.url,True)['assigned'],2)
        self.assertEqual(synchronize(self.url,True)['assigned'],0)
        self.assertEqual(self.conn.execute('SELECT sum(price) FROM public.agent_apis').fetchone()[0],14)
    def test_concurrent_scripts_assign_once(self):
        results=[];errors=[]
        def run():
            try:results.append(synchronize(self.url,True)['assigned'])
            except Exception as error:errors.append(error)
        threads=[threading.Thread(target=run) for _ in range(2)]
        for thread in threads:thread.start()
        for thread in threads:thread.join()
        self.assertFalse(errors);self.assertEqual(sorted(results),[0,2])
    def test_failure_rolls_back_whole_batch(self):
        self.conn.execute("CREATE FUNCTION reject_second() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id='00000000-0000-4000-8000-000000000002' THEN RAISE EXCEPTION 'test rejection'; END IF; RETURN NEW; END $$")
        self.conn.execute('CREATE TRIGGER reject_second BEFORE UPDATE ON public.agent_apis FOR EACH ROW EXECUTE FUNCTION reject_second()')
        with self.assertRaises(Exception):synchronize(self.url,True)
        self.assertEqual(self.conn.execute('SELECT count(*) FROM public.agent_apis WHERE public_mcp_tool_name IS NOT NULL').fetchone()[0],0)
        self.conn.execute('DROP FUNCTION reject_second() CASCADE')

    def seed_legacy_names(self):
        old = ['demo_get_list_00000000', 'demo_get_list_00000000000040008000000000000002']
        for identity, name in zip(self.ids, old):
            self.conn.execute('UPDATE public.agent_apis SET public_mcp_tool_name=%s WHERE id=%s', (name, identity))
        return old

    def test_rename_preview_apply_and_idempotence(self):
        old = self.seed_legacy_names()
        preview = synchronize(self.url, rename_generated=True)
        self.assertEqual(preview['renamed'], 2)
        self.assertEqual([item['previous_tool_name'] for item in preview['assignments']], old)
        self.assertEqual([r[0] for r in self.conn.execute('SELECT public_mcp_tool_name FROM public.agent_apis ORDER BY id')], old)
        applied = synchronize(self.url, True, rename_generated=True)
        self.assertEqual(applied['assignments'], preview['assignments'])
        self.assertEqual([r[0] for r in self.conn.execute('SELECT public_mcp_tool_name FROM public.agent_apis ORDER BY id')],
                         ['demo_get_list', 'demo_get_list_2'])
        self.assertEqual(synchronize(self.url, True, rename_generated=True)['assigned'], 0)
        self.assertEqual(synchronize(self.url, True)['assigned'], 0)
        self.assertEqual(self.conn.execute('SELECT sum(price) FROM public.agent_apis').fetchone()[0], 14)

    def test_rename_releases_old_names_before_final_assignment(self):
        self.conn.execute("UPDATE public.agent_apis SET path_template='/list_00000000', public_mcp_tool_name='demo_get_list_00000000_00000000' WHERE id=%s", (self.ids[0],))
        self.conn.execute("UPDATE public.agent_apis SET public_mcp_tool_name='demo_get_list_00000000' WHERE id=%s", (self.ids[1],))
        self.assertEqual(synchronize(self.url, True, rename_generated=True)['renamed'], 2)
        self.assertEqual([r[0] for r in self.conn.execute('SELECT public_mcp_tool_name FROM public.agent_apis ORDER BY id')],
                         ['demo_get_list_00000000', 'demo_get_list'])

    def test_failed_rename_restores_cleared_names(self):
        from psycopg.errors import RaiseException
        old = self.seed_legacy_names()
        self.conn.execute("CREATE FUNCTION reject_renamed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id='00000000-0000-4000-8000-000000000002' AND NEW.public_mcp_tool_name IS NOT NULL THEN RAISE EXCEPTION 'test rejection'; END IF; RETURN NEW; END $$")
        self.conn.execute('CREATE TRIGGER reject_renamed BEFORE UPDATE ON public.agent_apis FOR EACH ROW EXECUTE FUNCTION reject_renamed()')
        try:
            with self.assertRaises(RaiseException): synchronize(self.url, True, rename_generated=True)
            self.assertEqual([r[0] for r in self.conn.execute('SELECT public_mcp_tool_name FROM public.agent_apis ORDER BY id')], old)
        finally:
            self.conn.execute('DROP FUNCTION reject_renamed() CASCADE')

    def test_concurrent_renames_apply_once(self):
        self.seed_legacy_names()
        results = []; errors = []
        def run():
            try: results.append(synchronize(self.url, True, rename_generated=True)['renamed'])
            except Exception as error: errors.append(error)
        threads = [threading.Thread(target=run) for _ in range(2)]
        for thread in threads: thread.start()
        for thread in threads: thread.join()
        self.assertFalse(errors)
        self.assertEqual(sorted(results), [0, 2])
