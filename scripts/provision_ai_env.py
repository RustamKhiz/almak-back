"""Provision only AI settings via SSH stdin; never transmit the local database config."""
import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import sys
import tempfile

KEYS = ('AI_ORDER_DRAFTS_ENABLED', 'DEEPSEEK_API_KEY', 'DEEPSEEK_MODEL', 'DEEPSEEK_BASE_URL')


def rewrite_env(content, settings):
    if set(settings) != set(KEYS):
        raise ValueError('Unexpected AI settings')
    for value in settings.values():
        if not isinstance(value, str) or not value or any(c in value for c in '\r\n\x00'):
            raise ValueError('Invalid AI setting')
    # JSON double quoting is supported by godotenv and protects dotenv metacharacters.
    lines = [line for line in content.splitlines() if not re.match(r'^\s*(?:export\s+)?(' + '|'.join(KEYS) + r')\s*=', line)]
    return '\n'.join(lines).rstrip() + '\n' + '\n'.join(key + '=' + json.dumps(settings[key]) for key in KEYS) + '\n'


def env_path():
    info = subprocess.check_output(['systemctl', 'show', 'myapp-backend', '--property=WorkingDirectory,EnvironmentFiles'], text=True)
    props = dict(line.split('=', 1) for line in info.splitlines() if '=' in line)
    cwd = props.get('WorkingDirectory', '')
    candidates = []
    if cwd and cwd != '/' and Path(cwd).is_absolute():
        candidates.append(Path(cwd) / '.env')
    candidates.extend(Path(part) for part in shlex.split(props.get('EnvironmentFiles', '')) if part.startswith('/'))
    for path in candidates:
        if path.is_file():
            return path.resolve()
    raise RuntimeError('No existing environment file found for myapp-backend; configuration unchanged')


def root_command(*args):
    subprocess.run(['sudo', '-n', *args], check=True, stdout=subprocess.DEVNULL)


def apply():
    settings = json.load(sys.stdin)
    path = env_path()
    metadata = path.stat()
    # The server permits narrowly scoped rsync/mv operations, not sudo Python.
    # Read/write staging files stay on the server in a private temporary directory.
    with tempfile.TemporaryDirectory(prefix='almak-ai-env-') as directory:
        local = Path(directory) / 'environment'
        root_command('/usr/bin/rsync', '-a', '--numeric-ids', '--chmod=F600',
                     '--chown=%d:%d' % (os.getuid(), os.getgid()), str(path), str(local))
        original = local.read_text(encoding='utf-8')
        content = rewrite_env(original, settings)
        backup = path.with_name(path.name + '.before-ai')
        if not backup.exists():
            root_command('/usr/bin/rsync', '-a', '--chmod=F600', str(path), str(backup))
        local.write_text(content, encoding='utf-8')
        staged = path.with_name(path.name + '.ai-' + str(os.getpid()))
        try:
            root_command('/usr/bin/rsync', '-a', '--numeric-ids',
                         '--chmod=F%o' % (stat.S_IMODE(metadata.st_mode) & 0o640),
                         '--chown=%d:%d' % (metadata.st_uid, metadata.st_gid),
                         str(local), str(staged))
            root_command('/bin/mv', '--', str(staged), str(path))
        finally:
            if staged.exists():
                root_command('/bin/mv', '--', str(staged), str(Path(directory) / 'unused'))
    print('AI environment configured; existing application settings and ownership preserved')


def send():
    key = os.environ.get('DEEPSEEK_API_KEY', '').strip()
    if not key:
        print('DEEPSEEK_API_KEY secret is absent; server configuration unchanged')
        return
    settings = dict(zip(KEYS, ['true', key, 'deepseek-flash', 'https://api.deepseek.com']))
    target = os.environ['SERVER_USER'] + '@' + os.environ['SERVER_HOST']
    identity = str(Path.home() / '.ssh' / 'id_ed25519')
    remote = '/tmp/almak-provision-ai-env-' + str(os.getpid()) + '.py'
    ssh = ['ssh', '-i', identity, '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', target]
    subprocess.run(['scp', '-i', identity, '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', __file__, target + ':' + remote], check=True)
    try:
        subprocess.run(ssh + ['python3 ' + shlex.quote(remote) + ' --apply'], input=json.dumps(settings), text=True, check=True)
    finally:
        subprocess.run(ssh + ['rm -f -- ' + shlex.quote(remote)], check=False)


if __name__ == '__main__':
    try:
        apply() if sys.argv[1:] == ['--apply'] else send()
    except Exception as exc:
        # Do not include subprocess input or environment values in diagnostics.
        print('AI provisioning failed: ' + type(exc).__name__, file=sys.stderr)
        sys.exit(1)
