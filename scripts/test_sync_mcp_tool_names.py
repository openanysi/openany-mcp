import unittest
from sync_mcp_tool_names import assign_names

class NameTests(unittest.TestCase):
    def row(self, api_id, name=None, path='/api/v1/crypto/list'):
        return {'api_id': api_id, 'provider_id': 'blockrun', 'method': 'GET', 'path_template': path,
                'status': 'active', 'public_mcp_tool_name': name}
    def test_preserves_existing_names_and_generates_once(self):
        rows=[self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9'),self.row('00000000-0000-4000-8000-000000000001','manual_name')]
        result=assign_names(rows)
        self.assertEqual(result,[{'api_id':'e1183d75-4cb5-41b3-a351-0b027f17dea9','status':'active','tool_name':'blockrun_get_api_v1_crypto_list_e1183d75'}])
        rows[0]['public_mcp_tool_name']=result[0]['tool_name']
        self.assertEqual(assign_names(rows),[])
    def test_only_new_collision_extends_suffix(self):
        rows=[self.row('11111111-0000-4000-8000-000000000001','blockrun_get_api_v1_crypto_list_e1183d75'),self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9')]
        self.assertEqual(assign_names(rows)[0]['tool_name'],'blockrun_get_api_v1_crypto_list_e1183d754cb5')
    def test_empty_existing_name_is_not_overwritten(self):
        with self.assertRaises(ValueError):assign_names([self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9','')])
    def test_long_names_are_bounded_and_disabled_are_assigned(self):
        r=self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9',path='/'+'x'*200);r['status']='disabled'
        name=assign_names([r])[0]['tool_name'];self.assertLessEqual(len(name),64);self.assertTrue(name.endswith('_e1183d75'))

if __name__=='__main__':unittest.main()
