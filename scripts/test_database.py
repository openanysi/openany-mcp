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
