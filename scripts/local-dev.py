#!/usr/bin/env python3
"""Start, inspect, and stop Mythborn's local development services."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import time
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1]
STATE_DIR = ROOT / 'bin' / 'local-dev'
STATE_FILE = STATE_DIR / 'processes.json'
RUNTIME_KEYS = {
    'DATABASE_URL', 'SUPABASE_URL', 'SUPABASE_SERVICE_ROLE_KEY',
    'SUPABASE_PHOTO_BUCKET', 'GEMINI_API_KEY', 'TEMPORAL_ADDRESS',
    'TEMPORAL_NAMESPACE', 'TEMPORAL_API_KEY', 'CORS_ALLOWED_ORIGINS',
}


def application_env():
    env = os.environ.copy()
    for line in (ROOT / '.env').read_text().splitlines():
        if '=' not in line or line.lstrip().startswith('#'):
            continue
        key, value = line.split('=', 1)
        if key.strip() in RUNTIME_KEYS:
            # Read values as data: never execute .env as a shell script.
            env[key.strip()] = value.strip().strip("\"'")
    env['HTTP_ADDRESS'] = '127.0.0.1:8080'
    return env


def process_identity(pid):
    try:
        fields = Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()
        return None if fields[0] == 'Z' else fields[19]
    except (FileNotFoundError, ProcessLookupError):
        return None


def running(entry):
    return process_identity(entry['pid']) == entry['identity']


def load_state():
    return json.loads(STATE_FILE.read_text()) if STATE_FILE.exists() else {}


def save_state(state):
    STATE_FILE.write_text(json.dumps(state, indent=2) + '\n')


def port_open(port):
    try:
        with socket.create_connection(('127.0.0.1', port), timeout=0.5):
            return True
    except OSError:
        return False


def existing_service(name):
    """Recognize healthy app services already running outside this helper."""
    try:
        if name == 'api':
            with urlopen('http://127.0.0.1:8080/healthz', timeout=2) as response:
                if json.load(response).get('status') != 'ok':
                    return False
            with urlopen('http://127.0.0.1:8080/readyz', timeout=2) as response:
                return response.status == 204
        if name == 'web':
            with urlopen('http://127.0.0.1:5173/', timeout=2) as response:
                return '<title>Mythborn' in response.read(100000).decode()
    except (OSError, ValueError):
        pass
    return False


def wait_for_service(entry, url):
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if not running(entry):
            raise RuntimeError(f"Service stopped; inspect {entry['log']}")
        try:
            with urlopen(url, timeout=1) as response:
                if 200 <= response.status < 400:
                    return
        except OSError:
            pass
        time.sleep(0.25)
    raise RuntimeError(f"Service not ready at {url}; inspect {entry['log']}")


def start():
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    state = load_state()
    app_env = application_env()
    if app_env.get('TEMPORAL_ADDRESS') != 'localhost:7233' or app_env.get('TEMPORAL_NAMESPACE') != 'default' or app_env.get('TEMPORAL_API_KEY'):
        raise RuntimeError('Set .env to local Temporal: localhost:7233, namespace default, empty API key.')
    node = shutil.which('node')
    vite = ROOT / 'web' / 'node_modules' / 'vite' / 'bin' / 'vite.js'
    services = [
        ('temporal', [str(ROOT / 'bin' / 'temporal'), 'server', 'start-dev',
                      '--ip', '127.0.0.1', '--ui-ip', '127.0.0.1',
                      '--db-filename', str(STATE_DIR / 'temporal.db')],
         ROOT, os.environ.copy(), 7233, 'http://127.0.0.1:8233/'),
        ('worker', [str(ROOT / 'bin' / 'mythborn-worker')], ROOT, app_env, None, None),
        ('api', [str(ROOT / 'bin' / 'mythborn-api')], ROOT, app_env,
         8080, 'http://127.0.0.1:8080/readyz'),
    ]
    if not node or not vite.exists():
        raise RuntimeError('Install the web dependencies before starting: cd web && npm install')
    services.append(('web', [node, str(vite), '--host', '127.0.0.1', '--port', '5173', '--strictPort'],
                     ROOT / 'web', os.environ.copy(), 5173, 'http://127.0.0.1:5173/'))
    for name, command, cwd, env, port, url in services:
        if name in state and running(state[name]):
            print(f'{name}: already running (PID {state[name]["pid"]})', flush=True)
            if url:
                wait_for_service(state[name], url)
            continue
        if port and port_open(port):
            if existing_service(name):
                print(f'{name}: using existing healthy service on port {port}', flush=True)
                continue
            raise RuntimeError(f'Port {port} is already in use; no existing service was stopped.')
        if name == 'temporal' and port_open(8233):
            raise RuntimeError('Port 8233 is already in use; no existing service was stopped.')
        if not Path(command[0]).exists():
            raise RuntimeError(f'Missing executable: {command[0]}')
        log_path = STATE_DIR / f'{name}.log'
        with log_path.open('a') as log:
            process = subprocess.Popen(command, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                       stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        identity = process_identity(process.pid)
        if identity is None:
            raise RuntimeError(f'{name} stopped during launch; inspect {log_path}')
        entry = {'pid': process.pid, 'identity': identity, 'log': str(log_path)}
        state[name] = entry
        save_state(state)
        if url:
            wait_for_service(entry, url)
        else:
            time.sleep(1)
            if not running(entry):
                raise RuntimeError(f'{name} stopped during launch; inspect {log_path}')
        print(f'{name}: started (PID {process.pid})', flush=True)
    status()


def status():
    state = load_state()
    for name in ['temporal', 'worker', 'api', 'web']:
        entry = state.get(name)
        if entry and running(entry):
            detail = f'running (PID {entry["pid"]})'
        elif existing_service(name):
            detail = 'running outside this helper'
        else:
            detail = 'stopped'
        print(f'{name}: {detail}')
    print('App: http://localhost:5173')
    print('Temporal UI: http://localhost:8233')
    print(f'Logs and persistent Temporal database: {STATE_DIR}')


def stop():
    state = load_state()
    for name in ['web', 'api', 'worker', 'temporal']:
        entry = state.get(name)
        if entry and running(entry):
            os.killpg(entry['pid'], signal.SIGTERM)
            deadline = time.monotonic() + 10
            while running(entry) and time.monotonic() < deadline:
                time.sleep(0.1)
            if running(entry):
                raise RuntimeError(f'{name} did not stop within 10 seconds; retry stop.')
            print(f'{name}: stopped')
        if entry and not running(entry):
            state.pop(name)
    save_state(state)


def restart_backend():
    """Reload the API/worker while leaving Temporal's durable timers running."""
    state = load_state()
    external_api = None
    if port_open(8080) and not (state.get('api') and running(state['api'])):
        listeners = subprocess.check_output(['ss', '-ltnp', 'sport = :8080'], text=True)
        match = re.search(r'pid=(\d+)', listeners)
        if not match:
            raise RuntimeError('Cannot identify the API listener on port 8080.')
        pid = int(match.group(1))
        executable = os.readlink(f'/proc/{pid}/exe')
        cwd = Path(os.readlink(f'/proc/{pid}/cwd'))
        known_api = executable == str(ROOT / 'bin' / 'mythborn-api') or (executable.startswith('/tmp/go-build') and executable.endswith('/exe/api'))
        if cwd != ROOT or not known_api:
            raise RuntimeError('Port 8080 belongs to a different process; no service was stopped.')
        external_api = {'pid': pid, 'identity': process_identity(pid)}
    for name in ['worker', 'api']:
        entry = state.get(name)
        if entry and running(entry):
            os.killpg(entry['pid'], signal.SIGTERM)
            deadline = time.monotonic() + 10
            while running(entry) and time.monotonic() < deadline:
                time.sleep(0.1)
            if running(entry):
                raise RuntimeError(f'{name} did not stop within 10 seconds.')
        state.pop(name, None)
    save_state(state)
    if external_api and running(external_api):
        os.kill(external_api['pid'], signal.SIGTERM)
        deadline = time.monotonic() + 10
        while running(external_api) and time.monotonic() < deadline:
            time.sleep(0.1)
        if running(external_api):
            raise RuntimeError('The previous API did not stop within 10 seconds.')
    start()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['start', 'status', 'stop', 'restart-backend'])
    args = parser.parse_args()
    try:
        {'start': start, 'status': status, 'stop': stop, 'restart-backend': restart_backend}[args.action]()
    except (OSError, RuntimeError) as error:
        parser.exit(1, f'{error}\n')
