<?php

namespace app\api\controller;

use think\Controller;
use think\Db;
use think\Exception;
use think\facade\Config;
use app\common\model\Vod;

class Scan extends Controller
{
    protected $vodModel;

    public function __construct()
    {
        parent::__construct();
        $this->vodModel = new Vod();
    }

    /**
     * 检查并添加翻译字段
     */
    public function check_translate_field()
    {
        echo 1;exit;
        try {
            // 检查是否存在translated字段
            $prefix = config('database.prefix');
            $fields = Db::query("SHOW COLUMNS FROM {$prefix}vod");
            $hasTranslated = false;
            
            foreach ($fields as $field) {
                if ($field['Field'] === 'translated') {
                    $hasTranslated = true;
                    break;
                }
            }

            // 如果不存在translated字段，则添加
            if (!$hasTranslated) {
                Db::execute("ALTER TABLE {$prefix}vod ADD translated TINYINT(1) DEFAULT 0");
                return json(['code' => 1, 'msg' => '翻译字段添加成功']);
            }
            
            return json(['code' => 1, 'msg' => '翻译字段已存在']);
        } catch (Exception $e) {
            return json(['code' => 0, 'msg' => $e->getMessage()]);
        }
    }

    /**
     * 扫描并翻译电影
     */
    public function scan_and_translate()
    {
        try {
            // 先检查字段是否存在
            $this->check_translate_field();

            // 获取未翻译的视频
            $videos = $this->vodModel->where('translated', 0)->select();

            $successCount = 0;
            $failCount = 0;

            foreach ($videos as $video) {
                if (!empty($video['vod_name'])) {
                    // 调用翻译API
                    $translatedText = $this->translate_text($video['vod_name']);
                    
                    if ($translatedText) {
                        // 更新视频信息
                        $video->vod_name = $translatedText;
                        $video->translated = 1;
                        $video->save();
                        $successCount++;
                    } else {
                        $failCount++;
                    }
                }
            }

            return json([
                'code' => 1, 
                'msg' => "扫描完成，成功翻译: {$successCount} 条，失败: {$failCount} 条"
            ]);

        } catch (Exception $e) {
            return json(['code' => 0, 'msg' => $e->getMessage()]);
        }
    }

    /**
     * 调用翻译API
     */
    protected function translate_text($text)
    {
        try {
            $url = "http://47.83.131.112:8010/translate?text=" . urlencode($text);
            $response = file_get_contents($url);
            
            if ($response) {
                $result = json_decode($response, true);
                return isset($result['translated_text']) ? $result['translated_text'] : false;
            }
            
            return false;
        } catch (Exception $e) {
            return false;
        }
    }
}