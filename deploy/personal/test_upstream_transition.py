"""退出个人应用补丁：固定官方镜像、原账本保留和失败关闭。"""
import copy
import json
from pathlib import Path
import unittest

import deploy_hostdzire as deploy
from deploy_database import upstream_ledger_baseline, upstream_ledger_prefix
import maintenance_hostdzire
import test_deploy_hostdzire as fixture
import test_klno18_deployment as klno18

ROOT = Path(__file__).resolve().parents[2]
OLD = dict(deploy.UPSTREAM_OLD, image_id=fixture.OLD_ID)
NEW = dict(deploy.UPSTREAM_TARGET, input=deploy.UPSTREAM_TARGET['image'])
ADDITIONS = [
    b'247_scoped_admin_api_keys.sql|03445774e2dfad32da9e23279193d1bca02953896bc3d3b8a87ba0b36b6ed577\n',
    b'248_group_image_count_billing.sql|a872dde339b19f311feb5fa6fd85c8ff7b11d69794c35a18e2086d9846b3499c\n',
    b'254_add_usage_log_tps_inputs.sql|4171933399bdae34c818f4638b2c4576c0208eab75f2d6c39f19c6c085cf768c\n',
]


def ledger(prefix=3):
    return b''.join(sorted(klno18.ledger().splitlines(keepends=True) + ADDITIONS[:prefix]))


class UpstreamContractTests(unittest.TestCase):
    def test_only_fixed_official_digest_version_and_metadata_are_accepted(self):
        self.assertEqual(deploy.parse_target(NEW['image'], NEW['version']), NEW)
        deploy.validate_metadata(NEW)
        for value in ('ghcr.io/kln-4096/sub2api:latest', '0.2.19-klno.1', NEW['image'][:-1]+'b'):
            with self.subTest(value=value), self.assertRaises(deploy.DeployError):
                deploy.parse_target(value, NEW['version'])
        for key in ('image','revision','version','source'):
            invalid=dict(NEW);invalid[key]+='x'
            with self.subTest(key=key), self.assertRaises(deploy.DeployError): deploy.validate_metadata(invalid)

    def test_proof_binds_full_trees_tag_and_forbids_rollback_or_rehashed_changes(self):
        proof=deploy.GitEvidence(ROOT).compatibility(OLD,NEW)
        self.assertEqual(len(proof['changed_paths']),412)
        with self.assertRaises(deploy.DeployError): deploy.require_image_rollback(proof)
        for key in ('upstream_switch','changed_paths','upstream'):
            invalid=copy.deepcopy(proof)
            if key=='changed_paths':invalid[key].append('unknown')
            elif key=='upstream':invalid[key]['upstream_sha']='0'*40
            else:invalid[key]['image_rollback_compatible']=True
            invalid['sha256']=deploy.sha(deploy.canonical({k:v for k,v in invalid.items() if k!='sha256'}))
            with self.subTest(key=key), self.assertRaises(deploy.DeployError):
                deploy.validate_compatibility_proof(invalid,OLD['revision'],NEW['revision'],OLD['version'],NEW['version'])

    def test_all_prefixes_retain_old_rows_and_unknown_missing_or_changed_rows_fail(self):
        before=ledger(0)
        self.assertEqual(deploy.migration_fingerprint(before),deploy.UPSTREAM_OLD_LEDGER)
        for prefix in range(4):self.assertEqual(upstream_ledger_prefix(before,ledger(prefix)),prefix)
        self.assertEqual(upstream_ledger_baseline(ledger(),True),before)
        self.assertEqual(upstream_ledger_baseline(before,False),before)
        for invalid in (ledger(2),ledger()+b'unknown.sql|'+b'0'*64+b'\n',ledger().replace(b'247_scoped_admin',b'249_scoped_admin'),ledger().replace(b'99562d',b'000000')):
            with self.assertRaises(deploy.DeployError):upstream_ledger_baseline(invalid,True)


class UpstreamLifecycleTests(unittest.TestCase):
    setUp=fixture.DeploymentTests.setUp
    execute=fixture.DeploymentTests.execute
    persisted=fixture.DeploymentTests.persisted
    starts=fixture.DeploymentTests.starts

    def prepare(self):
        self.docker.images[OLD['image']]=self.docker.images[fixture.OLD_ID]=fixture.image(OLD,fixture.OLD_ID)
        self.docker.current=self.docker.config_image=OLD['image']
        self.docker.migration_listing=ledger(0)
        self.docker.migrations_on_start=ledger()
        self.docker.images[NEW['image']]=self.docker.images[fixture.NEW_ID]=fixture.image(NEW,fixture.NEW_ID)
        self.docker.failure_image=NEW['image']
        state=self.base/'.personal-deploy'; previous='20261025T044327Z-c82a22085b08'
        deploy.private_directory(state/'records'/previous)
        deploy.atomic_private(state/'records'/previous/'record.json',deploy.canonical({'id':previous,'status':'success','action':'deploy','target':OLD,'migration_state':deploy.migration_fingerprint(ledger(0))}))
        deploy.atomic_private(state/'image.env',('SUB2API_IMAGE='+OLD['image']+'\nSUB2API_DEPLOYMENT_ID='+previous+'\n').encode())
        value=fixture.request(NEW,OLD)
        value['compatibility']=deploy.GitEvidence(ROOT).compatibility(OLD,NEW)
        return value,previous

    def test_backup_precedes_start_success_commits_fixed_digest_and_resume_record(self):
        value,_=self.prepare();env=(self.base/'.env').read_bytes()
        result,code=self.execute(value)
        self.assertEqual((code,result['status']),(0,'success'))
        saved=self.persisted(result)
        self.assertEqual(saved['migration_state']['rows'],319)
        self.assertEqual(saved['migration_committed_prefix'],3)
        self.assertEqual((self.base/'.env').read_bytes(),env)
        self.assertEqual(deploy.Deployment(self.base,self.docker).selected()['image'],NEW['image'])
        commands=[args for args,_ in self.docker.commands]
        self.assertLess(next(i for i,args in enumerate(commands) if 'pg_dump' in args[-1]),next(i for i,args in enumerate(commands) if '--no-deps' in args))
        self.assertEqual(maintenance_hostdzire.maintenance_record(saved,saved['id'],NEW['revision']),saved['target'])
        saved['target']['image']=OLD['image']
        with self.assertRaises((deploy.DeployError,RuntimeError)):maintenance_hostdzire.maintenance_record(saved,saved['id'],NEW['revision'])

    def test_partial_migration_or_health_failure_keeps_selection_and_never_restarts_old(self):
        for prefix in range(4):
            with self.subTest(prefix=prefix):
                self.setUp();value,previous=self.prepare()
                self.docker.migrations_on_start=ledger(prefix);self.docker.wait_failure=True
                result,code=self.execute(value)
                self.assertEqual((code,result['rollback_status']),(1,'blocked'))
                self.assertEqual(len(self.starts()),1)
                self.assertEqual(self.docker.current,NEW['image'])
                self.assertEqual(deploy.Deployment(self.base,self.docker).selected()['record_id'],previous)
                self.assertEqual(self.persisted(result)['migration_committed_prefix'],prefix)

    def test_backup_failure_never_starts_application(self):
        value,_=self.prepare();self.docker.backup_failure=True
        result,code=self.execute(value)
        self.assertEqual(code,1);self.assertEqual(self.starts(),[])


if __name__=='__main__':unittest.main()
