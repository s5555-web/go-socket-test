#!/usr/bin/env python3
"""Encrypt only this project's SQL backups. --apply removes sources after verification.

Verification uses a sha256 manifest; decrypt and compare on the administrator's
computer before invoking --apply. The recovery private key never goes on server.
"""
import argparse, hashlib, json, os, subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--apply', action='store_true');p.add_argument('--certificate',required=True);args=p.parse_args()
root=Path('/opt/signal-web/backups')
manifest_path=root/'sql-encryption-manifest.json'
if args.apply:
    manifest=json.loads(manifest_path.read_text())
    for item in manifest:
        source=Path(item['source']);encrypted=Path(item['encrypted'])
        if root not in source.parents or source.is_symlink() or root not in encrypted.parents:raise RuntimeError('unsafe backup path')
        if not encrypted.is_file():raise RuntimeError('missing encrypted backup')
        if source.exists():
            if hashlib.sha256(source.read_bytes()).hexdigest()!=item['sha256']:raise RuntimeError('backup changed; stop')
            source.unlink()
    print('plaintext SQL backups removed; encrypted backups retained:',len(manifest))
else:
    manifest=[]
    for source in sorted(root.rglob('*')):
        if not source.is_file() or source.is_symlink() or not (source.name.endswith('.sql') or source.name.endswith('.sql.gz') or source.name.endswith('.sql.xz')):continue
        encrypted=Path(str(source)+'.cms')
        if encrypted.exists():raise RuntimeError('encrypted backup already exists; inspect before retry')
        subprocess.run(['openssl','cms','-encrypt','-binary','-aes-256-gcm','-in',str(source),'-outform','DER','-out',str(encrypted),args.certificate],check=True)
        os.chmod(encrypted,0o600)
        manifest.append({'source':str(source),'encrypted':str(encrypted),'sha256':hashlib.sha256(source.read_bytes()).hexdigest()})
    manifest_path.write_text(json.dumps(manifest,indent=2));os.chmod(manifest_path,0o600)
    print('encrypted SQL backups awaiting off-server verification:',len(manifest))
