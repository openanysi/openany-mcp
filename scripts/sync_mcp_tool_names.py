#!/usr/bin/env python3
"""Preview or transactionally fill missing public MCP names. Never rename existing values."""
import argparse
import json
import os
import re
import sys
import uuid

NAME = re.compile(r'[a-z][a-z0-9_]{0,63}\Z', re.ASCII)
LOCK_NAMESPACE = 741216
LOCK_KEY = 1


def assign_names(rows):
    occupied = set()
    for row in rows:
        name = row['public_mcp_tool_name']
        if name is not None:
            if not NAME.fullmatch(name) or name in occupied:
                raise ValueError('Invalid or duplicate existing public name')
            occupied.add(name)
    assigned = []
    for row in sorted(rows, key=lambda row: str(row['api_id'])):
        if row['public_mcp_tool_name'] is not None:
            continue
        identity = uuid.UUID(str(row['api_id'])).hex
        source = f"{row['provider_id']}_{row['method']}_{row['path_template']}".lower()
        prefix = re.sub(r'[^a-z0-9]+', '_', source, flags=re.ASCII).strip('_')
        for length in (8, 12, 16, 32):
            candidate = prefix[:63-length].rstrip('_') + '_' + identity[:length]
            if NAME.fullmatch(candidate) and candidate not in occupied:
                occupied.add(candidate)
                assigned.append({'api_id': str(row['api_id']), 'status': row['status'], 'tool_name': candidate})
                break
        else:
            raise ValueError('Cannot assign a unique name for API ' + str(row['api_id']))
    return assigned


def check_schema(conn):
    column = conn.execute("""SELECT data_type,is_nullable,column_default FROM information_schema.columns
      WHERE table_schema='public' AND table_name='agent_apis' AND column_name='public_mcp_tool_name'""").fetchone()
    if column != {'data_type': 'text', 'is_nullable': 'YES', 'column_default': None}:
        raise ValueError('Required nullable text column missing or incompatible; run the migration')
    constraint = conn.execute("""SELECT pg_get_constraintdef(oid) AS definition FROM pg_constraint
      WHERE conrelid='public.agent_apis'::regclass AND conname='agent_apis_public_mcp_tool_name_format_check'
      AND contype='c' AND convalidated""").fetchone()
    expected = "CHECKpublic_mcp_tool_nameISNULLORpublic_mcp_tool_name~'^[a-z][a-z0-9_]{0,63}$'::text"
    if not constraint or re.sub(r'\s+|[()]', '', constraint['definition']) != expected:
        raise ValueError('Required name format constraint missing or incompatible')
    index = conn.execute("""SELECT i.indisunique AND i.indisvalid AND i.indisready AND i.indpred IS NULL
      AND i.indexprs IS NULL AND i.indnatts=1 AND i.indnkeyatts=1
      AND i.indkey[0]=a.attnum AND NOT i.indnullsnotdistinct AS valid
      FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
      JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attname='public_mcp_tool_name'
      WHERE i.indrelid='public.agent_apis'::regclass AND c.relname='agent_apis_public_mcp_tool_name_unique'""").fetchone()
    if not index or not index['valid']:
        raise ValueError('Required global nullable unique index missing or incompatible')


def synchronize(database_url, apply=False):
    import psycopg
    from psycopg.rows import dict_row
    with psycopg.connect(database_url, autocommit=True, row_factory=dict_row,
                         application_name='openany-mcp-name-sync', connect_timeout=5) as conn:
        with conn.transaction():
            conn.execute("SET LOCAL lock_timeout='5s'")
            conn.execute("SET LOCAL statement_timeout='30s'")
            if not apply:
                conn.execute('SET TRANSACTION READ ONLY')
            check_schema(conn)
            if apply:
                conn.execute('SELECT pg_advisory_xact_lock(%s,%s)', (LOCK_NAMESPACE, LOCK_KEY))
                conn.execute('SELECT id FROM public.agent_apis WHERE public_mcp_tool_name IS NULL ORDER BY id FOR UPDATE').fetchall()
            rows = conn.execute('''SELECT id::text AS api_id,provider_id,method,path_template,status,public_mcp_tool_name
                                 FROM public.agent_apis ORDER BY id''').fetchall()
            assignments = assign_names(rows)
            if apply:
                for item in assignments:
                    cursor = conn.execute('''UPDATE public.agent_apis SET public_mcp_tool_name=%s,updated_at=now()
                      WHERE id=%s AND public_mcp_tool_name IS NULL''', (item['tool_name'], item['api_id']))
                    if cursor.rowcount != 1:
                        raise ValueError('Concurrent name change; transaction rolled back')
            result = {'mode': 'apply' if apply else 'preview', 'total': len(rows),
                      'already_named': len(rows)-len(assignments), 'assigned': len(assignments),
                      'committed': apply, 'assignments': assignments}
        return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true', help='commit assignments; default is read-only preview')
    args = parser.parse_args()
    database_url = os.environ.get('POSTGRES_DATABASE_URL')
    if not database_url:
        print(json.dumps({'error': 'POSTGRES_DATABASE_URL is required'}), file=sys.stderr)
        return 1
    try:
        result = synchronize(database_url, args.apply)
    except ValueError as error:
        print(json.dumps({'error': str(error)}), file=sys.stderr)
        return 1
    except Exception as error:
        # Driver exceptions can contain connection credentials or server details.
        print(json.dumps({'error': 'Database operation failed; no partial assignments committed',
                          'category': type(error).__name__}), file=sys.stderr)
        return 1
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
