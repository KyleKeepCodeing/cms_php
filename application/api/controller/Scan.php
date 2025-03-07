<?php

namespace app\api\controller;

use think\Controller;


// Api增加一个扫库的文件Scan.php 然后实现扫库电影，扫库前检测电影库有没有是否翻译的字段 如果没有则创建字段默认为0
// 然后扫描=0的电影并调用api接口翻译
// 接口是
// @http://47.83.131.112:8010/translate?text=翻译的中文
// 然后返回的字段是json 里面的translated_text是翻译的内容
class Scan extends Controller
{
    public function check_translate_field()
    {
        echo 1;exit;
    
   
    }
}