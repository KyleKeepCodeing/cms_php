<?php
namespace app\index\controller;

use think\Db;

class Index extends Base
{
    public function __construct()
    {
        parent::__construct();
    }

    public function arraySort($array,$keys,$sort='asc') {
        $newArr = $valArr = array();
        foreach ($array as $key=>$value) {
            $valArr[$key] = $value[$keys];
        }
        ($sort == 'asc') ?  asort($valArr) : arsort($valArr);
        reset($valArr);
        foreach($valArr as $key=>$value) {
            $newArr[$key] = $array[$key];
        }
        return $newArr;
    }

    public function index()
    {
        $config = config('mctheme');
        $config = $config['theme']['type']['hom'];
        $config_atr = explode(',',$config);
        // 最新影视
        $vod_types = Db::table('mac_type')->whereIn('type_id',$config_atr)->where('type_pid',0)->column('type_name','type_id');
        $lists = [];
        foreach ($vod_types as $index => $item) {
            $vod_list = Db::table('mac_type')
                ->where('type_pid',$index)
                ->column('type_name','type_id');
            $lists[$index]['vod_type_name'] = $item;
            $lists[$index]['vod_son_name_list'] = $vod_list;
        }

        $time = strtotime('-1 day');
        $time_str = date('Y-m-d',$time);
        $time_start = strtotime($time_str);
        $time_end = $time_start+48*60*60-1;

        foreach ($lists as $index => $list) {
            $vod_count_num = 0;

            if ($list['vod_son_name_list'] != []) {

                foreach ($list['vod_son_name_list'] as $i => $item) {

                    $vod = Db::table('mac_vod')
                        ->where('type_id',$i)
                        ->order('vod_time_add desc')
                        ->limit(9)
                        ->select();

                    $vod_count = Db::table('mac_vod')
                        ->where('type_id',$i)
                        ->where('vod_time_add','between',[$time_start,$time_end])
                        ->count();

                    foreach($vod as $ii => $kk){
                        $lists[$index]['vod_detail_array'][] = $kk;
                    }
                    $vod_count_num += $vod_count;
                }
                $lists[$index]['vod_detail_array'] = array_slice($this->arraySort($lists[$index]['vod_detail_array'],'vod_time_add','desc'),0,9);
                $lists[$index]['vod_count'] = $vod_count_num;
            }else{
                    $vod = Db::table('mac_vod')
                        ->where('type_id',$index)
                        ->order('vod_time_add desc')
                        ->limit(9)
                        ->select();

                    $vod_count = Db::table('mac_vod')
                        ->where('type_id',$index)
                        ->where('vod_time_add','between',[$time_start,$time_end])
                        ->count();

                    foreach($vod as $ii => $kk){
                        $lists[$index]['vod_detail_array'][] = $kk;
                    }

                    $vod_count_num += $vod_count;

                $lists[$index]['vod_count'] = $vod_count_num;
            }
        }

        $count_num_1 = model('art')->count();

        $count = floor($count_num_1/3)*3;

        if ($count > 3) {
            $count = 3;
        }

        // 查询是否存在文章分类

        $art_type_array = model('type')->where(['type_mid' => 2 ,'type_status' => 1])->column('type_id');

        $models = model('art')->whereIn('type_id',$art_type_array)->order('art_time desc')->limit($count*3)->select();

        $model_array = [];

        foreach ($models as $index => $model) {
            array_push($model_array,$model->toArray());
        }

        $config = config('maccms');

        $tips_text = $config['site']['site_announcement'];

        return $this->label_fetch('index/index',1,'html',[
            'models' => $model_array,
            'last_updates' => $lists,
            'count_num_1' => $count_num_1,
            'tips_text' => $tips_text,
            'art_type_array' => $art_type_array,
        ]);
    }

}
