"""直接执行维护控制器的隔离文件/替身命令合同，不访问实际站点或 VPS。"""
import contextlib, hashlib, io, json, tempfile, types, unittest
from pathlib import Path
SOURCE=Path(__file__).with_name('maintenance_hostdzire.py')
class GateTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.base=Path(self.tmp.name).resolve()
  self.g=types.ModuleType('gate');exec(compile(SOURCE.read_bytes(),str(SOURCE),'exec'),self.g.__dict__)
  self.g.BASE=self.base/'app';self.g.SITE=self.base/'nginx'/'api.conf';self.g.SITE.parent.mkdir();self.g.SITE.write_bytes(b'installed gate\n')
  self.g.ALLOWED_SITE_PARENTS={self.g.SITE.parent};self.g.FLAG=self.base/self.g.FLAG.relative_to('/');self.g.FLAG_DIRECTORY=self.g.FLAG.parent;self.g.FLAG_DIRECTORY.mkdir(parents=True,mode=0o755);self.g.DIRECTORY=self.g.BASE/'.personal-deploy/rollouts/rel002'
  self.g.DIRECTORY.mkdir(parents=True);(self.g.BASE/'.personal-deploy/deployment.lock').touch()
  self.record={'source_sha':self.g.SOURCE_SHA,'site':str(self.g.SITE),'before_sha256':'unused','after_sha256':self.g.sha(self.g.SITE.read_bytes()),'site_mode':0o600,'phase':'ready'}
  self.g.save(self.record)
  self.image='ghcr.io/ccisnoxx/sub2api@sha256:'+'a'*64;self.id='20261012T180000Z-123456789abc'
  state=self.g.BASE/'.personal-deploy';(state/'records'/self.id).mkdir(parents=True)
  (state/'image.env').write_text('SUB2API_IMAGE='+self.image+'\nSUB2API_DEPLOYMENT_ID='+self.id+'\n')
  self.ledger=b'\n'.join(('migration_%03d|checksum'%n).encode() for n in range(308))+b'\n'
  self.deployed={'status':'success','target':{'revision':self.g.SOURCE_SHA,'image':self.image},'migration_state':{'rows':308,'filename_checksum_sha256':self.g.sha(self.ledger)}}
  self.g.atomic(state/'records'/self.id/'record.json',json.dumps(self.deployed).encode())
  self.app={'Config':{'Image':self.image,'Labels':{'org.opencontainers.image.revision':self.g.SOURCE_SHA}},'State':{'Running':True,'Health':{'Status':'healthy'}}}
  self.calls=[]
  def run(args,**kw):
   self.calls.append(args)
   if args[:2]==['docker','inspect']:return json.dumps([self.app]).encode()
   if args[0]=='curl':return b'{"status":"ok"}'
   if args[:2]==['docker','exec']:return self.ledger
   raise AssertionError('意外的服务器命令')
  self.g.run=run
 def action(self,name):
  with contextlib.redirect_stdout(io.StringIO()):self.g.main(name)
 def phase(self):return json.loads((self.g.DIRECTORY/'record.json').read_bytes())['phase']
 def test_freeze_resume_refreeze_and_each_resume_revalidates(self):
  self.action('freeze');self.assertTrue(self.g.FLAG.exists());self.assertEqual(self.phase(),'closed')
  self.action('resume');self.assertFalse(self.g.FLAG.exists());self.assertEqual(self.phase(),'open')
  self.action('freeze');self.assertTrue(self.g.FLAG.exists());self.assertEqual(self.phase(),'closed')
  self.app['State']['Health']['Status']='unhealthy'
  with self.assertRaises(RuntimeError):self.action('resume')
  self.assertTrue(self.g.FLAG.exists());self.assertEqual(self.phase(),'closed')
  self.app['State']['Health']['Status']='healthy';self.action('resume')
  self.assertFalse(self.g.FLAG.exists());self.assertEqual(sum(x[:2]==['docker','inspect'] for x in self.calls),3)
 def test_opening_without_flag_can_close_after_interruption(self):
  self.record['phase']='opening';self.g.save(self.record)
  self.action('freeze');self.assertTrue(self.g.FLAG.exists());self.assertEqual(self.phase(),'closed')
 def test_unknown_flag_and_site_drift_are_not_overwritten(self):
  self.g.FLAG.write_bytes(b'other operator')
  with self.assertRaises(RuntimeError):self.action('freeze')
  self.assertEqual(self.g.FLAG.read_bytes(),b'other operator')
  self.g.FLAG.unlink();self.g.SITE.write_bytes(b'drift')
  with self.assertRaises(RuntimeError):self.action('freeze')
  self.assertEqual(self.g.SITE.read_bytes(),b'drift')
 def test_closed_marker_survives_volatile_directory_reset_and_controller_restart(self):
  self.action('freeze');stored=self.g.FLAG.read_bytes()
  volatile=self.base/'run';volatile.mkdir(exist_ok=True);(volatile/'irrelevant.flag').write_bytes(b'temporary')
  import shutil
  shutil.rmtree(volatile);volatile.mkdir()
  restarted=types.ModuleType('restart');exec(compile(SOURCE.read_bytes(),str(SOURCE),'exec'),restarted.__dict__)
  for name in ('BASE','SITE','ALLOWED_SITE_PARENTS','FLAG_DIRECTORY','FLAG','DIRECTORY','run'):setattr(restarted,name,getattr(self.g,name))
  self.g=restarted
  self.assertTrue(self.g.FLAG.exists());self.assertEqual(self.g.FLAG.read_bytes(),stored);self.assertEqual(self.phase(),'closed')
  self.action('freeze');self.assertEqual(self.g.FLAG.read_bytes(),stored)
 def test_marker_directory_permission_drift_and_symlink_are_rejected(self):
  self.g.FLAG_DIRECTORY.chmod(0o700)
  with self.assertRaises(RuntimeError):self.action('freeze')
  self.assertFalse(self.g.FLAG.exists())
  self.g.FLAG_DIRECTORY.chmod(0o755);self.g.FLAG_DIRECTORY.rmdir();self.g.FLAG_DIRECTORY.symlink_to(self.g.BASE,target_is_directory=True)
  with self.assertRaises(RuntimeError):self.action('freeze')
 def test_migration_drift_keeps_public_closed(self):
  self.action('freeze');self.ledger+=b'unknown|checksum\n'
  with self.assertRaises(RuntimeError):self.action('resume')
  self.assertTrue(self.g.FLAG.exists());self.assertEqual(self.phase(),'closed')
if __name__=='__main__':unittest.main()
