import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch
import tempfile
import shutil
import json
import io

spec = importlib.util.spec_from_file_location('provision', Path(__file__).with_name('provision_ai_env.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class ProvisionTests(unittest.TestCase):
    def test_preserves_database_and_replaces_only_ai(self):
        settings = dict(zip(module.KEYS, ['true', 'synthetic-key', 'deepseek-flash', 'https://api.deepseek.com']))
        value = module.rewrite_env('DB_PASSWORD="unchanged"\nJWT_SECRET=unchanged\nexport DEEPSEEK_API_KEY=old\n', settings)
        self.assertIn('DB_PASSWORD="unchanged"\nJWT_SECRET=unchanged', value)
        self.assertEqual(value.count('DEEPSEEK_API_KEY='), 1)
        self.assertNotIn('=old', value)
    def test_rejects_line_injection(self):
        settings = dict(zip(module.KEYS, ['true', 'key\nDB_PASSWORD=bad', 'deepseek-flash', 'https://api.deepseek.com']))
        with self.assertRaises(ValueError): module.rewrite_env('DB_PASSWORD=original', settings)
    def test_is_idempotent(self):
        settings = dict(zip(module.KEYS, ['true', 'synthetic-key', 'deepseek-flash', 'https://api.deepseek.com']))
        value = module.rewrite_env('DB_NAME=orders_db\n', settings)
        self.assertEqual(module.rewrite_env(value, settings), value)

class ApplyTests(unittest.TestCase):
    def test_preserves_existing_settings_and_backup_during_atomic_replacement(self):
        settings = dict(zip(module.KEYS, ['true', 'synthetic-key', 'deepseek-flash', 'https://api.deepseek.com']))
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / '.env'
            original = 'DB_PASSWORD=unchanged\nJWT_SECRET=unchanged\n'
            path.write_text(original, encoding='utf-8')
            def permitted(*args):
                if args[0] == '/usr/bin/rsync': shutil.copy2(args[-2], args[-1])
                elif args[0] == '/bin/mv': shutil.move(args[-2], args[-1])
                else: self.fail('Unexpected privileged command')
            with patch.object(module, 'env_path', return_value=path), patch.object(module, 'root_command', side_effect=permitted), patch.object(module.os, 'getuid', return_value=0, create=True), patch.object(module.os, 'getgid', return_value=0, create=True), patch.object(module.sys, 'stdin', io.StringIO(json.dumps(settings))):
                module.apply()
            self.assertIn(original.strip(), path.read_text(encoding='utf-8'))
            self.assertIn('DEEPSEEK_API_KEY="synthetic-key"', path.read_text(encoding='utf-8'))
            self.assertEqual(path.with_name('.env.before-ai').read_text(encoding='utf-8'), original)
            self.assertEqual(sorted(p.name for p in Path(directory).iterdir()), ['.env', '.env.before-ai'])

if __name__ == '__main__': unittest.main()
