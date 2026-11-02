import contextlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import provision_team


class TeamProvisioningTests(unittest.TestCase):
    def setUp(self):
        for name in ('getuid', 'getgid'):
            mock = patch('provision_team.os.' + name, return_value=1000, create=True)
            mock.start()
            self.addCleanup(mock.stop)

    def test_private_server_config_and_redacted_result(self):
        team = {'adminLogin': 'almak', 'managers': [{'login': 'manager1', 'password': 'private-first-value'}, {'login': 'manager2', 'password': 'private-second-value'}]}
        profiles = [{'id': 1, 'login': 'almak', 'role': 'admin'}, {'id': 2, 'login': 'manager1', 'role': 'manager'}]
        with tempfile.TemporaryDirectory() as directory:
            env = Path(directory) / 'server.env'
            env.write_text('DB_PASSWORD=private-server-value', encoding='utf-8')
            staging = []

            def copy(*args):
                target = Path(args[-1])
                staging.append(target.parent)
                shutil.copyfile(args[-2], target)

            def run(args, **kwargs):
                self.assertNotIn('private-first-value', ' '.join(args))
                self.assertEqual(kwargs['timeout'], 60)
                self.assertEqual(json.loads(kwargs['input']), team)
                self.assertEqual((Path(kwargs['cwd']) / '.env').read_text(encoding='utf-8'), env.read_text(encoding='utf-8'))
                return subprocess.CompletedProcess(args, 0, stdout=json.dumps(profiles), stderr='')

            output = io.StringIO()
            with patch('provision_ai_env.env_path', return_value=env), patch('provision_ai_env.root_command', side_effect=copy), patch('provision_team.subprocess.run', side_effect=run), patch('sys.stdin', io.StringIO(json.dumps(team))), contextlib.redirect_stdout(output):
                provision_team.apply(str(Path(directory) / 'almak-users'))
            self.assertEqual(json.loads(output.getvalue()), profiles)
            self.assertNotIn('private-', output.getvalue())
            self.assertTrue(all(not path.exists() for path in staging))
            self.assertEqual(env.read_text(encoding='utf-8'), 'DB_PASSWORD=private-server-value')

    def test_failed_command_does_not_echo_subprocess_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            env=Path(directory)/'server.env'
            env.write_text('test',encoding='utf-8')
            with patch('provision_ai_env.env_path',return_value=env), patch('provision_ai_env.root_command'), patch('provision_team.subprocess.run',return_value=subprocess.CompletedProcess([],1,stdout='private-password',stderr='private-password')), patch('sys.stdin',io.StringIO('{}')):
                with self.assertRaisesRegex(RuntimeError,'no account changes committed'):
                    provision_team.apply(str(Path(directory)/'almak-users'))


if __name__ == '__main__':
    unittest.main()
