"""Provision users through SSH stdin, using only the server's own DB config."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile


def apply(binary):
    from provision_ai_env import env_path, root_command
    team = json.load(sys.stdin)
    with tempfile.TemporaryDirectory(prefix='almak-users-config-') as directory:
        config = Path(directory) / '.env'
        root_command('/usr/bin/rsync', '-a', '--numeric-ids', '--chmod=F600',
                     '--chown=%d:%d' % (os.getuid(), os.getgid()), str(env_path()), str(config))
        result = subprocess.run([str(Path(binary).resolve()), '-action', 'provision-team'],
                                input=json.dumps(team), text=True, cwd=directory,
                                capture_output=True, check=False, timeout=60)
        if result.returncode:
            raise RuntimeError('User provisioning failed; no account changes committed')
        # Validate and restrict output; neither subprocess diagnostics nor secrets are echoed.
        profiles = json.loads(result.stdout)
        print(json.dumps([{'id': p['id'], 'login': p['login'], 'role': p['role']} for p in profiles]))


def send(binary):
    raw = os.environ.get('INITIAL_TEAM_ACCOUNTS', '')
    if not raw:
        raise RuntimeError('INITIAL_TEAM_ACCOUNTS secret is required')
    team = json.loads(raw)
    target = os.environ['SERVER_USER'] + '@' + os.environ['SERVER_HOST']
    identity = str(Path.home() / '.ssh' / 'id_ed25519')
    ssh = ['ssh', '-i', identity, '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', target]
    result = subprocess.run(ssh + ['mktemp -d /tmp/almak-users-XXXXXXXX'], capture_output=True, text=True, check=True)
    directory = result.stdout.strip()
    if not directory.startswith('/tmp/almak-users-') or '/' in directory[len('/tmp/'):]:
        raise RuntimeError('Unexpected staging path')
    local = Path(__file__).resolve().parent
    files = [Path(binary).resolve(), Path(__file__).resolve(), local / 'provision_ai_env.py']
    try:
        subprocess.run(['scp', '-i', identity, '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes',
                        *map(str, files), target + ':' + directory + '/'], check=True, stdout=subprocess.DEVNULL)
        command = 'python3 ' + shlex.quote(directory + '/provision_team.py') + ' --apply ' + shlex.quote(directory + '/' + files[0].name)
        subprocess.run(ssh + [command], input=json.dumps(team), text=True, check=True)
    finally:
        names = [directory + '/' + p.name for p in files]
        # Importing the helper must not leave __pycache__ behind.
        subprocess.run(ssh + ['rm -f -- ' + ' '.join(shlex.quote(n) for n in names) + '; rmdir -- ' + shlex.quote(directory)], check=False, stdout=subprocess.DEVNULL)


if __name__ == '__main__':
    sys.dont_write_bytecode = True
    try:
        apply(sys.argv[2]) if sys.argv[1:2] == ['--apply'] else send(sys.argv[1])
    except Exception:
        print('User provisioning failed; credentials were not logged. Check server access and account configuration.', file=sys.stderr)
        sys.exit(1)
