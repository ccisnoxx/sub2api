"""REL-002 单站点维护入口；服务器执行，配置备份只留在私密记录。"""
import fcntl, hashlib, json, os, pathlib, stat, subprocess, sys
BASE = pathlib.Path('/root/sub2api-kin')
SITE = pathlib.Path('/etc/nginx/sites-enabled/api.962850.xyz.conf')
ALLOWED_SITE_PARENTS = {pathlib.Path('/etc/nginx/sites-enabled'),pathlib.Path('/etc/nginx/sites-available')}
ORIGINAL_SHA = '66c146e9e0750a728866b73c1580e362d098aa0a0b7955bb419c01465d5cbfca'
SOURCE_SHA = '42907bcab278f93db48d7e1b39b28e28f664569e'
# 标记属于持久状态，不能放在重启清空的 /run；Nginx worker 只需遍历和读取。
FLAG_DIRECTORY = pathlib.Path('/var/lib/sub2api-rel002-maintenance')
FLAG = FLAG_DIRECTORY/'closed'
DIRECTORY = BASE/'.personal-deploy/rollouts/REL-002-42907bcab278'
MARKER = b'    if (-f /var/lib/sub2api-rel002-maintenance/closed) { return 503; }\n'
FLAG_CONTENT = b'REL-002 42907bcab278\n'
def require(value, message):
 if not value: raise RuntimeError(message)
def sha(value):return hashlib.sha256(value).hexdigest()
def run(args, **kw):
 result=subprocess.run(args,capture_output=True,**kw)
 if result.returncode:
  if DIRECTORY.exists():atomic(DIRECTORY/'command-failure.log',json.dumps({'command':args,'exit':result.returncode}).encode()+b'\n'+result.stdout+b'\n'+result.stderr)
  raise RuntimeError('服务器命令失败，详情保留在私密记录')
 return result.stdout
def regular(path):
 require(path.is_file() and not path.is_symlink(),'操作文件必须是普通文件')
 return path.read_bytes()
def atomic(path, value, mode=0o600):
 require(not path.is_symlink(),'禁止覆盖符号链接')
 temp=path.with_name(path.name+'.rel002-tmp')
 require(not temp.exists() and not temp.is_symlink(),'临时文件已经存在，停止覆盖')
 with temp.open('xb') as f:
  os.chmod(temp,mode);f.write(value);f.flush();os.fsync(f.fileno())
 os.replace(temp,path)
 fd=os.open(path.parent,os.O_DIRECTORY)
 try:os.fsync(fd)
 finally:os.close(fd)
def save(record):atomic(DIRECTORY/'record.json',(json.dumps(record,sort_keys=True)+'\n').encode())
def nginx():run(['nginx','-t']);run(['systemctl','reload','nginx'])
def flag_directory(create=False):
 if create:
  require(not FLAG_DIRECTORY.exists() and not FLAG_DIRECTORY.is_symlink(),'维护标记目录已经存在，停止接管')
  FLAG_DIRECTORY.mkdir(mode=0o755)
  FLAG_DIRECTORY.chmod(0o755)
  fd=os.open(FLAG_DIRECTORY.parent,os.O_DIRECTORY)
  try:os.fsync(fd)
  finally:os.close(fd)
 require(FLAG_DIRECTORY.is_dir() and not FLAG_DIRECTORY.is_symlink(),'维护标记目录不是普通目录')
 info=FLAG_DIRECTORY.stat()
 require(info.st_uid==os.geteuid() and info.st_gid==os.getegid() and stat.S_IMODE(info.st_mode)==0o755,'维护标记目录所有权或权限漂移')

def execute(action):
 require(action in {'setup','freeze','resume','remove'},'未知维护操作')
 actual=SITE.resolve(strict=True)
 require(actual.parent in ALLOWED_SITE_PARENTS,'站点真实路径漂移')
 if action=='setup':
  require(not DIRECTORY.exists() and not FLAG.exists() and not FLAG.is_symlink(),'维护记录或标记已经存在')
  before=regular(actual)
  require(sha(before)==ORIGINAL_SHA and before.count(b'\nserver {\n')==2 and before.count(b'server_name api.962850.xyz;')==2,'站点内容与已核实现场不同')
  run(['nginx','-t'])
  flag_directory(create=True)
  DIRECTORY.mkdir(parents=True,mode=0o700);DIRECTORY.chmod(0o700)
  (DIRECTORY.parent).chmod(0o700)
  mode=stat.S_IMODE(actual.stat().st_mode)
  require(actual.stat().st_uid==0 and actual.stat().st_gid==0,'站点所有权不是 root')
  after=before.replace(b'\nserver {\n',b'\nserver {\n'+MARKER)
  atomic(DIRECTORY/'site-before.conf',before)
  record={'source_sha':SOURCE_SHA,'site':str(actual),'before_sha256':sha(before),'after_sha256':sha(after),'site_mode':mode,'phase':'setting_up'}
  save(record)
  require(sha(regular(actual))==ORIGINAL_SHA,'设置前站点发生漂移')
  atomic(actual,after,mode)
  try:nginx()
  except Exception:
   require(sha(regular(actual))==record['after_sha256'],'设置失败后站点漂移，停止覆盖')
   atomic(actual,before,mode)
   run(['nginx','-t']);run(['systemctl','reload','nginx'])
   record['phase']='setup_failed';save(record);raise
  record['phase']='ready';save(record)
 else:
  flag_directory()
  record=json.loads(regular(DIRECTORY/'record.json'))
  require(record['source_sha']==SOURCE_SHA and record['site']==str(actual) and sha(regular(actual))==record['after_sha256'],'维护记录或站点发生漂移')
  if action=='freeze':
   require(record['phase'] in {'ready','open','opening','closed'} and not FLAG.is_symlink(),'维护状态不允许冻结')
   if FLAG.exists():require(regular(FLAG)==FLAG_CONTENT,'维护标记不属于本次操作')
   else:atomic(FLAG,FLAG_CONTENT,0o644)
   record['phase']='closed';save(record)
  elif action=='resume':
   require(record['phase']=='closed' and regular(FLAG)==FLAG_CONTENT,'维护状态不允许恢复')
   selection=dict(x.split('=',1) for x in regular(BASE/'.personal-deploy/image.env').decode().splitlines())
   deployment_id=selection['SUB2API_DEPLOYMENT_ID']
   import re
   require(re.fullmatch(r'[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}',deployment_id),'部署记录身份无效')
   deployed=json.loads(regular(BASE/'.personal-deploy/records'/deployment_id/'record.json'))
   app=json.loads(run(['docker','inspect','sub2api-kin']))[0]
   require(deployed['status']=='success' and deployed['target']['revision']==SOURCE_SHA and selection['SUB2API_IMAGE']==deployed['target']['image']==app['Config']['Image'],'成功记录或运行镜像不匹配')
   require(app['State']['Running'] and app['State']['Health']['Status']=='healthy' and app['Config']['Labels']['org.opencontainers.image.revision']==SOURCE_SHA,'新应用健康或 revision 不匹配')
   health=json.loads(run(['curl','--fail','--silent','--max-time','10','http://127.0.0.1:10088/health']))
   require(health['status']=='ok','回环健康检查未通过')
   ledger=run(['docker','exec','sub2api-kin-postgres','sh','-ec','psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT filename,checksum FROM schema_migrations ORDER BY filename,checksum"'])
   require({'rows':len(ledger.splitlines()),'filename_checksum_sha256':sha(ledger)}==deployed['migration_state'] and len(ledger.splitlines())==308,'当前迁移账本与成功记录不同')
   record['accepted_deployment_id']=deployment_id;record['phase']='opening';save(record)
   FLAG.unlink()
   try:record['phase']='open';save(record)
   except Exception:atomic(FLAG,FLAG_CONTENT,0o644);raise
  else:
   require(record['phase']=='open' and not FLAG.exists() and not FLAG.is_symlink(),'流量尚未恢复，禁止移除维护入口')
   before=regular(DIRECTORY/'site-before.conf');require(sha(before)==record['before_sha256']==ORIGINAL_SHA,'站点备份摘要不符')
   after=regular(actual);atomic(actual,before,record['site_mode'])
   try:nginx()
   except Exception:
    require(sha(regular(actual))==ORIGINAL_SHA,'移除失败后站点漂移，停止覆盖')
    atomic(actual,after,record['site_mode']);nginx();raise
   record['phase']='complete';save(record)
 print(json.dumps({'action':action,'phase':record['phase'],'record':str(DIRECTORY),'source_sha':SOURCE_SHA,'maintenance_flag_present':FLAG.exists()}))
def main(action):
 fd=os.open(BASE/'.personal-deploy/deployment.lock',os.O_CREAT|os.O_RDWR|os.O_NOFOLLOW,0o600)
 with os.fdopen(fd,'a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  execute(action)
if __name__=='__main__':
 try:main(sys.argv[1])
 except Exception as e:
  print(json.dumps({'status':'failed','type':type(e).__name__}));sys.exit(1)
