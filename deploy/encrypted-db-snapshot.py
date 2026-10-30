#!/usr/bin/env python3
"""Stream this project's database backup directly into AES-GCM CMS ciphertext."""
import os, subprocess, sys
from pathlib import Path
certificate, destination=sys.argv[1:]
dsn=os.environ['SIGNAL_DATABASE_DSN']
auth,sep,location=dsn.partition('@tcp(')
if not sep:raise RuntimeError('unsupported DSN; refusing unsafe fallback')
user,password=auth.split(':',1)
address,dbname=location.split(')/',1);dbname=dbname.split('?',1)[0]
if dbname!='signal_web':raise RuntimeError('unexpected database; refusing backup')
host,port=address.rsplit(':',1)
env=dict(os.environ,MYSQL_PWD=password)
with open(destination,'xb') as output:
    dump=subprocess.Popen(['mysqldump','--host='+host,'--port='+port,'--user='+user,'--single-transaction','--skip-lock-tables','--no-tablespaces','--set-gtid-purged=OFF',dbname],stdout=subprocess.PIPE,env=env)
    encrypt=subprocess.Popen(['openssl','cms','-encrypt','-binary','-aes-256-gcm','-outform','DER',certificate],stdin=dump.stdout,stdout=output)
    dump.stdout.close()
    encrypted_status=encrypt.wait();dump_status=dump.wait()
    if encrypted_status or dump_status:raise RuntimeError('encrypted snapshot failed; do not deploy')
os.chmod(destination,0o600)
print('encrypted database snapshot ready')
