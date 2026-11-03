import unittest
from sync_mcp_tool_names import assign_names


class NameTests(unittest.TestCase):
    def row(self, api_id, name=None, path='/api/v1/crypto/list'):
        return {'api_id': api_id, 'provider_id': 'blockrun', 'method': 'GET', 'path_template': path,
                'status': 'active', 'public_mcp_tool_name': name}

    def test_preserves_existing_names_and_generates_once(self):
        rows = [self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9'),
                self.row('00000000-0000-4000-8000-000000000001', 'manual_name')]
        result = assign_names(rows)
        self.assertEqual(result, [{'api_id': 'e1183d75-4cb5-41b3-a351-0b027f17dea9',
                                  'status': 'active', 'tool_name': 'blockrun_get_api_v1_crypto_list'}])
        self.assertIsNone(rows[0]['public_mcp_tool_name'])
        rows[0]['public_mcp_tool_name'] = result[0]['tool_name']
        self.assertEqual(assign_names(rows), [])

    def test_user_example_has_no_suffix(self):
        row = self.row('300d7717-0000-4000-8000-000000000001', path='/v3/backlinks/summary/live')
        row.update(provider_id='dataforseo', method='POST')
        self.assertEqual(assign_names([row])[0]['tool_name'], 'dataforseo_post_v3_backlinks_summary_live')

    def test_collision_skips_reserved_numeric_aliases_and_disabled_names(self):
        rows = [self.row('00000000-0000-4000-8000-000000000001', 'blockrun_get_api_v1_crypto_list'),
                self.row('00000000-0000-4000-8000-000000000002', 'blockrun_get_api_v1_crypto_list_2'),
                self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9')]
        rows[1]['status'] = 'disabled'
        self.assertEqual(assign_names(rows)[0]['tool_name'], 'blockrun_get_api_v1_crypto_list_3')

    def test_batch_collisions_are_deterministic_without_displacing_saved_names(self):
        rows = [self.row('00000000-0000-4000-8000-000000000003'),
                self.row('00000000-0000-4000-8000-000000000002'),
                self.row('00000000-0000-4000-8000-000000000001')]
        expected = ['blockrun_get_api_v1_crypto_list', 'blockrun_get_api_v1_crypto_list_2',
                    'blockrun_get_api_v1_crypto_list_3']
        self.assertEqual([item['tool_name'] for item in assign_names(rows)], expected)
        self.assertEqual(assign_names(rows), assign_names(list(reversed(rows))))
        rows[0]['public_mcp_tool_name'] = 'blockrun_get_api_v1_crypto_list'
        self.assertEqual([item['tool_name'] for item in assign_names(rows)], expected[1:])

    def test_normal_run_keeps_legacy_names_and_rename_is_explicit(self):
        row = self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9', 'blockrun_get_api_v1_crypto_list_e1183d75')
        self.assertEqual(assign_names([row]), [])
        self.assertEqual(assign_names([row], rename_generated=True), [{
            'api_id': row['api_id'], 'status': 'active', 'tool_name': 'blockrun_get_api_v1_crypto_list',
            'previous_tool_name': 'blockrun_get_api_v1_crypto_list_e1183d75'}])

    def test_rename_recognizes_every_legacy_suffix_length(self):
        for suffix in ['e1183d75', 'e1183d754cb5', 'e1183d754cb541b3', 'e1183d754cb541b3a3510b027f17dea9']:
            with self.subTest(suffix=suffix):
                row = self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9', 'blockrun_get_api_v1_crypto_list_' + suffix)
                self.assertEqual(assign_names([row], rename_generated=True)[0]['tool_name'],
                                 'blockrun_get_api_v1_crypto_list')

    def test_rename_preserves_manual_names_even_when_they_look_generated(self):
        rows = [self.row('00000000-0000-4000-8000-000000000001', 'blockrun_get_api_v1_crypto_list'),
                self.row('00000000-0000-4000-8000-000000000002', 'blockrun_get_api_v1_crypto_list_deadbeef'),
                self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9', 'blockrun_get_api_v1_crypto_list_e1183d75')]
        result = assign_names(rows, rename_generated=True)
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]['tool_name'], 'blockrun_get_api_v1_crypto_list_2')
        rows[2]['public_mcp_tool_name'] = result[0]['tool_name']
        self.assertEqual(assign_names(rows, rename_generated=True), [])

    def test_empty_or_duplicate_existing_names_are_not_overwritten(self):
        for name in ['', 'same_name']:
            rows = [self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9', name)]
            if name: rows.append(self.row('00000000-0000-4000-8000-000000000001', name))
            with self.assertRaises(ValueError): assign_names(rows, rename_generated=True)

    def test_long_names_leave_room_for_multi_digit_aliases(self):
        rows = []
        base = 'blockrun_get_' + 'x' * 51
        for i in range(1, 10):
            name = base if i == 1 else base[:62] + '_' + str(i)
            rows.append(self.row('00000000-0000-4000-8000-%012d' % i, name))
        row = self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9', path='/' + 'x' * 200)
        row['status'] = 'disabled'
        rows.append(row)
        result = assign_names(rows)[0]
        self.assertEqual(result['tool_name'], 'blockrun_get_' + 'x' * 48 + '_10')
        self.assertEqual(len(result['tool_name']), 64)
        self.assertEqual(result['status'], 'disabled')

    def test_rename_recognizes_truncated_legacy_prefix(self):
        row = self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9',
                       'blockrun_get_' + 'x' * 42 + '_e1183d75', path='/' + 'x' * 200)
        self.assertEqual(assign_names([row], rename_generated=True)[0]['tool_name'],
                         'blockrun_get_' + 'x' * 51)

    def test_invalid_prefix_fails_instead_of_searching_aliases_forever(self):
        row = self.row('e1183d75-4cb5-41b3-a351-0b027f17dea9')
        row['provider_id'] = '123'
        with self.assertRaises(ValueError): assign_names([row])


if __name__ == '__main__':
    unittest.main()
