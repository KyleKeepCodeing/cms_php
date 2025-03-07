<?php

namespace app\api\controller;

use think\Controller;
use think\Db;
use think\Exception;
use think\facade\Config;

class Scan extends Controller
{
    protected $movieModel;

    public function __construct()
    {
        parent::__construct();
        // 设置数据库配置
        $config = [
            'type'     => env('database_type', 'mysql'),
            'hostname' => env('database_host', '127.0.0.1'),
            'database' => env('database_name', ''),
            'username' => env('database_user', ''),
            'password' => env('database_password', ''),
            'hostport' => env('database_port', '3306'),
            'charset'  => env('database_charset', 'utf8'),
            'prefix'   => env('database_table_prefix', 'mac_'),
        ];
        
        Db::connect($config);
        $this->movieModel = Db::name('movie');
    }

    /**
     * 检查并添加翻译字段
     */
    public function checkTranslateField()
    {
        try {
            // 检查是否存在translated字段
            $fields = Db::query("SHOW COLUMNS FROM " . Config::get('database.prefix') . "movie");
            $hasTranslated = false;
            
            foreach ($fields as $field) {
                if ($field['Field'] === 'translated') {
                    $hasTranslated = true;
                    break;
                }
            }

            // 如果不存在translated字段，则添加
            if (!$hasTranslated) {
                Db::execute("ALTER TABLE " . Config::get('database.prefix') . "movie ADD translated TINYINT(1) DEFAULT 0");
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
    public function scanAndTranslate()
    {
        try {
            // 先检查字段是否存在
            $this->checkTranslateField();

            // 获取未翻译的电影
            $movies = $this->movieModel
                ->where('translated', 0)
                ->select();

            $successCount = 0;
            $failCount = 0;

            foreach ($movies as $movie) {
                if (!empty($movie['name'])) {
                    // 调用翻译API
                    $translatedText = $this->translateText($movie['name']);
                    
                    if ($translatedText) {
                        // 更新电影信息
                        $this->movieModel->where('id', $movie['id'])->update([
                            'name' => $translatedText,
                            'translated' => 1
                        ]);
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
    protected function translateText($text)
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