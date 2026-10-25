"""REL-002 单站点维护入口；服务器执行，配置备份只留在私密记录。"""
import argparse, fcntl, hashlib, json, os, pathlib, re, stat, subprocess, sys
from deploy_config import load_config
from deploy_common import RECORD_RE, SHA_RE, validate_metadata
from deploy_compatibility import (OBS_NEW_VERSION, RESP_NEW_VERSION, KLNO4_NEW_VERSION, KLNO17_NEW_VERSION, KLNO18_NEW_VERSION,
                                KLNO17_OLD_MIGRATION_STATE, KLNO17_MIGRATIONS, KLNO18_OLD_MIGRATION_STATE, KLNO18_MIGRATIONS,
                                KLNO4_OLD_MIGRATION_STATE, KLNO4_MIGRATIONS, validate_compatibility_proof,
                                require_obs_old_image, require_resp_old_image, require_klno4_old_image, require_klno17_old_image, require_klno18_old_image)
from deploy_database import kin_ledger_baseline, klno4_ledger_baseline, klno17_ledger_baseline, klno18_ledger_baseline
CONFIG = load_config()
BASE = pathlib.Path(CONFIG.base)
SITE = pathlib.Path(CONFIG.maintenance.site)
ALLOWED_SITE_PARENTS = {pathlib.Path(value) for value in CONFIG.maintenance.allowed_site_parents}
ORIGINAL_SHA = '66c146e9e0750a728866b73c1580e362d098aa0a0b7955bb419c01465d5cbfca'
SOURCE_SHA = '42907bcab278f93db48d7e1b39b28e28f664569e'
# 标记属于持久状态，不能放在重启清空的 /run；Nginx worker 只需遍历和读取。
FLAG_DIRECTORY = pathlib.Path(CONFIG.maintenance.flag_directory)
FLAG = FLAG_DIRECTORY/'closed'
DIRECTORY = BASE/CONFIG.maintenance.record_directory
MARKER = ('    if (-f '+str(FLAG)+') { return 503; }\n').encode()
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
def private_binding_file(path):
 require(path.parent.is_dir() and not path.parent.is_symlink(),'OBS 部署记录目录不是普通目录')
 value=regular(path);info=path.stat()
 require(stat.S_IMODE(info.st_mode)==0o600 and info.st_uid==os.geteuid(),
         'OBS 部署选择或成功记录权限/所有权漂移')
 return value
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

def closed_flag(explicit):
 require(regular(FLAG)==FLAG_CONTENT,'维护标记不属于本次操作')
 if explicit:
  info=FLAG.stat()
  require(stat.S_IMODE(info.st_mode)==0o644 and info.st_uid==os.geteuid() and info.st_gid==os.getegid(),'维护标记权限或所有权漂移')

def maintenance_record(deployed, deployment_id, target_revision):
 require(deployed.get('schema')==1 and deployed.get('id')==deployment_id and deployed.get('action')=='deploy' and
         deployed.get('status')=='success' and deployed.get('exit_code')==0 and deployed.get('health_verified') is True,
         '维护恢复必须绑定本次健康验收并提交成功的部署记录')
 target=deployed['target'];old=deployed['old'];proof=deployed['compatibility']
 validate_metadata(target)
 require(target['revision']==target_revision and
         ((target['version']==OBS_NEW_VERSION and 'obs_upgrade' in proof) or
          (target['version']==RESP_NEW_VERSION and 'resp_upgrade' in proof) or
          (target['version']==KLNO4_NEW_VERSION and 'klno4_upgrade' in proof) or
          (target['version']==KLNO17_NEW_VERSION and 'klno17_upgrade' in proof) or
          (target['version']==KLNO18_NEW_VERSION and 'klno18_upgrade' in proof)),
         '维护恢复目标不属于审定发布')
 validate_compatibility_proof(proof,old['revision'],target_revision,old['version'],target['version'])
 require_obs_old_image(proof,old)
 require_resp_old_image(proof,old)
 require_klno4_old_image(proof,old)
 require_klno17_old_image(proof,old)
 require_klno18_old_image(proof,old)
 observed=deployed['observed_running']
 require(all(observed.get(key)==target[key] for key in ('image','image_id','revision','version','source')) and
         observed.get('running') is True and observed.get('health')=='healthy','维护恢复成功记录的观察终态不匹配')
 return target

def execute(action, target_revision=None, deployment_id=None):
 require(action in {'setup','freeze','resume','remove'},'未知维护操作')
 explicit=target_revision is not None or deployment_id is not None
 require(not explicit or (action=='resume' and SHA_RE.fullmatch(target_revision or '') and
                         RECORD_RE.fullmatch(deployment_id or '')),'恢复目标与记录必须完整且只用于 resume')
 actual=SITE.resolve(strict=True)
 require(actual.parent in ALLOWED_SITE_PARENTS,'站点真实路径漂移')
 if action=='setup':
  require(not DIRECTORY.exists() and not FLAG.exists() and not FLAG.is_symlink(),'维护记录或标记已经存在')
  before=regular(actual)
  require(sha(before)==ORIGINAL_SHA and before.count(b'\nserver {\n')==2 and before.count(('server_name '+CONFIG.maintenance.server_name+';').encode())==2,'站点内容与已核实现场不同')
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
  record_before=regular(DIRECTORY/'record.json');record=json.loads(record_before)
  require(record['source_sha']==SOURCE_SHA and record['site']==str(actual) and sha(regular(actual))==record['after_sha256'],'维护记录或站点发生漂移')
  if action=='freeze':
   require(record['phase'] in {'ready','open','opening','closed'} and not FLAG.is_symlink(),'维护状态不允许冻结')
   if FLAG.exists():require(regular(FLAG)==FLAG_CONTENT,'维护标记不属于本次操作')
   else:atomic(FLAG,FLAG_CONTENT,0o644)
   record['phase']='closed';save(record)
  elif action=='resume':
   require(record['phase']=='closed','维护状态不允许恢复');closed_flag(explicit)
   selection_path=BASE/CONFIG.state_directory/'image.env'
   read_binding=private_binding_file if explicit else regular
   selection_before=read_binding(selection_path)
   selected=re.fullmatch(r'SUB2API_IMAGE=(\S+)\nSUB2API_DEPLOYMENT_ID=([^\n]+)\n',selection_before.decode())
   require(selected is not None and RECORD_RE.fullmatch(selected[2]),'部署选择格式或记录身份无效')
   require(not explicit or deployment_id==selected[2],'显式成功部署记录与当前选择不匹配')
   deployment_id=selected[2];expected_revision=target_revision if explicit else SOURCE_SHA
   deployed_path=BASE/CONFIG.state_directory/'records'/deployment_id/'record.json'
   deployed_before=read_binding(deployed_path);deployed=json.loads(deployed_before)
   target=maintenance_record(deployed,deployment_id,target_revision) if explicit else deployed['target']
   app=json.loads(run(['docker','inspect',CONFIG.app_container]))[0]
   require(deployed['status']=='success' and target['revision']==expected_revision and selected[1]==target['image']==app['Config']['Image'],'成功记录或运行镜像不匹配')
   require(app['State']['Running'] is True and app['State']['Health']['Status']=='healthy' and app['Config']['Labels']['org.opencontainers.image.revision']==expected_revision,'新应用健康或 revision 不匹配')
   if explicit:
    labels=app['Config']['Labels']
    require(app['Image']==target['image_id'] and labels['org.opencontainers.image.version']==target['version'] and
            labels['org.opencontainers.image.source']==target['source'] and
            labels['com.docker.compose.project']==CONFIG.project and labels['com.docker.compose.service']==CONFIG.app_service,
            '维护恢复运行容器身份或 OCI 标签不匹配')
   health=json.loads(run(['curl','--fail','--silent','--max-time','10',CONFIG.health_url]))
   require(health['status']=='ok','回环健康检查未通过')
   ledger=run(['docker','exec',CONFIG.postgres_container,'sh','-ec','psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT filename,checksum FROM schema_migrations ORDER BY filename,checksum"'])
   expected_rows=(KLNO18_OLD_MIGRATION_STATE['rows']+len(KLNO18_MIGRATIONS)
                  if explicit and 'klno18_upgrade' in deployed['compatibility'] else
                  KLNO17_OLD_MIGRATION_STATE['rows']+len(KLNO17_MIGRATIONS)
                  if explicit and 'klno17_upgrade' in deployed['compatibility'] else
                  KLNO4_OLD_MIGRATION_STATE['rows']+len(KLNO4_MIGRATIONS)
                  if explicit and 'klno4_upgrade' in deployed['compatibility'] else 308)
   require({'rows':len(ledger.splitlines()),'filename_checksum_sha256':sha(ledger)}==deployed['migration_state'] and len(ledger.splitlines())==expected_rows,'当前迁移账本与成功记录不同')
   if explicit:
    if 'klno18_upgrade' in deployed['compatibility']:klno18_ledger_baseline(ledger,True)
    elif 'klno17_upgrade' in deployed['compatibility']:klno17_ledger_baseline(ledger,True)
    elif 'klno4_upgrade' in deployed['compatibility']:klno4_ledger_baseline(ledger,True)
    else:kin_ledger_baseline(ledger,True)
   flag_directory();closed_flag(explicit)
   require(SITE.resolve(strict=True)==actual and sha(regular(actual))==record['after_sha256'] and
           regular(DIRECTORY/'record.json')==record_before and read_binding(selection_path)==selection_before and
           read_binding(deployed_path)==deployed_before,'恢复验收期间站点、维护记录或部署选择发生漂移')
   record['accepted_deployment_id']=deployment_id;record['accepted_source_sha']=expected_revision;record['phase']='opening';save(record)
   FLAG.unlink()
   try:
    fd=os.open(FLAG_DIRECTORY,os.O_DIRECTORY)
    try:os.fsync(fd)
    finally:os.close(fd)
    record['phase']='open';save(record)
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
 print(json.dumps({'action':action,'phase':record['phase'],'record':str(DIRECTORY),'source_sha':SOURCE_SHA,
                   'accepted_source_sha':record.get('accepted_source_sha'),'accepted_deployment_id':record.get('accepted_deployment_id'),
                   'maintenance_flag_present':FLAG.exists()}))
def main(action, target_revision=None, deployment_id=None):
 fd=os.open(BASE/CONFIG.state_directory/'deployment.lock',os.O_CREAT|os.O_RDWR|os.O_NOFOLLOW,0o600)
 with os.fdopen(fd,'a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  execute(action,target_revision,deployment_id)
if __name__=='__main__':
 try:
  parser=argparse.ArgumentParser(description='固定 REL-002 站点维护；维护恢复显式绑定成功记录与目标 SHA')
  parser.add_argument('action',choices=('setup','freeze','resume','remove'))
  parser.add_argument('--target-revision');parser.add_argument('--deployment-record')
  args=parser.parse_args()
  require(args.target_revision is None and args.deployment_record is None or globals().get('__tool_attestation__') is not None,
          '维护恢复必须通过可信 main 完整工具 bundle 执行')
  main(args.action,args.target_revision,args.deployment_record)
 except Exception as e:
  print(json.dumps({'status':'failed','type':type(e).__name__}));sys.exit(1)
