#!/usr/bin/env python3
"""Run a project maintenance tool with systemd's literal environment file."""
import os,sys,subprocess
from pathlib import Path
env=dict(os.environ)
for line in Path('/etc/signal-web/secrets.env').read_text().splitlines():
    if not line.strip() or line.lstrip().startswith('#'):continue
    key,sep,value=line.partition('=')
    if sep:env[key.strip()]=value.strip().strip('"').strip("'")
sys.exit(subprocess.run(sys.argv[1:],env=env).returncode)
