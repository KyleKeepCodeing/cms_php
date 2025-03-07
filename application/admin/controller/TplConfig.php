<?php
namespace app\admin\controller;
use think\Db;

class TplConfig extends Base{

    public function theme() {
        if (Request()->isPost()) {
            $tplconfig = input();
            $tplconfig['theme']['fnav']['ym'] = join('|', $tplconfig['theme']['fnav']['ym']);
            $tplconfig['theme']['rtnav']['ym'] = join('|', $tplconfig['theme']['rtnav']['ym']);
            $tplconfig['theme']['show']['filter'] = join('|', $tplconfig['theme']['show']['filter']);
            $tplconfig_new['theme'] = $tplconfig['theme'];
            $tplconfig_old = config('mctheme');
            $tplconfig_new = array_merge($tplconfig_old, $tplconfig_new);
            $res = mac_save_config_data(APP_PATH . 'extra/mctheme.php', $tplconfig_new);
            if ($res === false) {
                return $this->error('保存失败，请重试!');
            }
            return $this->success('保存成功!');

        }

        $tplconfig = config('mctheme');
        $this->assign('tplconfig', $tplconfig);
        $this->assign('title', 'MACCMS PRO主题设置');
        return $this->fetch('admin@tplconfig/theme');
    }
}
