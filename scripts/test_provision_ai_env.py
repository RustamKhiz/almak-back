import importlib.util
from pathlib import Path
import unittest

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

if __name__ == '__main__': unittest.main()
