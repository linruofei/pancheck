import { useEffect, useState } from 'react';
import { toast } from 'sonner';

import { settingsApi, type MemoryConfig, type PlatformRateConfig } from '@/api/settingsApi';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { PLATFORM_NAMES } from '@/utils/constants';

const platforms = ['quark', 'uc', 'baidu', 'tianyi', 'pan123', 'pan115', 'aliyun', 'xunlei', 'cmcc'];

const defaultRateConfig: PlatformRateConfig = {
  enabled: true,
  concurrency: 5,
  request_delay_ms: 0,
  max_requests_per_second: 0,
};

export function Settings() {
  const [rateConfigSettings, setRateConfigSettings] = useState<Record<string, PlatformRateConfig>>({});
  const [memoryConfig, setMemoryConfig] = useState<MemoryConfig>({
    history_ttl_minutes: 2880,
    cleanup_interval_minutes: 10,
  });
  const [loading, setLoading] = useState(false);
  const [savingRate, setSavingRate] = useState(false);
  const [savingMemory, setSavingMemory] = useState(false);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const [rateConfig, memoryConfigData] = await Promise.all([
        settingsApi.getRateConfigSettings(),
        settingsApi.getMemoryConfig(),
      ]);
      setRateConfigSettings(rateConfig);
      setMemoryConfig(memoryConfigData);
    } catch (error: any) {
      toast.error(`加载设置失败: ${error.response?.data?.error || error.message}`);
    } finally {
      setLoading(false);
    }
  };

  const handleSaveMemoryConfig = async () => {
    if (memoryConfig.history_ttl_minutes < 1 || memoryConfig.history_ttl_minutes > 43200) {
      toast.error('历史数据保留时间必须在 1-43200 分钟之间');
      return;
    }
    if (memoryConfig.cleanup_interval_minutes < 1 || memoryConfig.cleanup_interval_minutes > 1440) {
      toast.error('清理间隔必须在 1-1440 分钟之间');
      return;
    }

    setSavingMemory(true);
    try {
      await settingsApi.updateMemoryConfig(memoryConfig);
      toast.success('保存成功，清理配置已生效');
    } catch (error: any) {
      toast.error(`保存失败: ${error.response?.data?.error || error.message}`);
    } finally {
      setSavingMemory(false);
    }
  };

  const handleSaveRateConfig = async () => {
    for (const [platform, config] of Object.entries(rateConfigSettings)) {
      if (config.concurrency < 1 || config.concurrency > 100) {
        toast.error(`${PLATFORM_NAMES[platform] || platform} 的并发数必须在 1-100 之间`);
        return;
      }
      if (config.request_delay_ms < 0 || config.request_delay_ms > 10000) {
        toast.error(`${PLATFORM_NAMES[platform] || platform} 的请求间隔必须在 0-10000 毫秒之间`);
        return;
      }
      if (config.max_requests_per_second < 0 || config.max_requests_per_second > 100) {
        toast.error(`${PLATFORM_NAMES[platform] || platform} 的每秒最大请求数必须在 0-100 之间`);
        return;
      }
    }

    setSavingRate(true);
    try {
      await settingsApi.updateRateConfigSettings(rateConfigSettings);
      toast.success('保存成功，配置已立即生效');
    } catch (error: any) {
      toast.error(`保存失败: ${error.response?.data?.error || error.message}`);
    } finally {
      setSavingRate(false);
    }
  };

  const updateRateConfig = (platform: string, field: keyof PlatformRateConfig, value: any) => {
    setRateConfigSettings((prev) => ({
      ...prev,
      [platform]: {
        ...(prev[platform] || defaultRateConfig),
        [field]: value,
      },
    }));
  };

  return (
    <div className="container mx-auto py-8 space-y-8">
      <Card>
        <CardHeader>
          <CardTitle>内存数据清理</CardTitle>
          <CardDescription>
            检测缓存和失效链接只保存在当前进程内存里，服务重启后会清空。
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-center py-8">加载中...</div>
          ) : (
            <div className="space-y-6">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="history-ttl">历史数据保留时间（分钟）</Label>
                  <Input
                    id="history-ttl"
                    type="number"
                    min="1"
                    max="43200"
                    value={memoryConfig.history_ttl_minutes}
                    onChange={(event) =>
                      setMemoryConfig({
                        ...memoryConfig,
                        history_ttl_minutes: parseInt(event.target.value, 10) || 2880,
                      })
                    }
                  />
                  <p className="text-sm text-muted-foreground">默认 2880 分钟，也就是 2 天。</p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="cleanup-interval">清理间隔（分钟）</Label>
                  <Input
                    id="cleanup-interval"
                    type="number"
                    min="1"
                    max="1440"
                    value={memoryConfig.cleanup_interval_minutes}
                    onChange={(event) =>
                      setMemoryConfig({
                        ...memoryConfig,
                        cleanup_interval_minutes: parseInt(event.target.value, 10) || 10,
                      })
                    }
                  />
                  <p className="text-sm text-muted-foreground">后台定时任务按这个间隔检查并删除历史数据。</p>
                </div>
              </div>

              <div className="flex justify-end">
                <Button onClick={handleSaveMemoryConfig} disabled={savingMemory} className="w-32">
                  {savingMemory ? '保存中...' : '保存配置'}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>频率控制配置</CardTitle>
          <CardDescription>配置每个网盘平台的请求频率限制，避免请求过多导致检测失败。</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-center py-8">加载中...</div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                {platforms.map((platform) => {
                  const platformConfig = rateConfigSettings[platform] || defaultRateConfig;
                  return (
                    <Card key={platform} className="p-4">
                      <div className="space-y-3">
                        <div className="font-semibold text-base mb-3 border-b pb-2">
                          {PLATFORM_NAMES[platform] || platform}
                        </div>

                        <div className="space-y-3">
                          <div className="flex items-center justify-between rounded-md border p-2">
                            <Label htmlFor={`${platform}-enabled`} className="text-sm">
                              是否启用检测
                            </Label>
                            <Switch
                              id={`${platform}-enabled`}
                              checked={platformConfig.enabled ?? true}
                              onCheckedChange={(checked) => updateRateConfig(platform, 'enabled', checked)}
                            />
                          </div>

                          <div className="space-y-1.5">
                            <Label htmlFor={`${platform}-concurrency`} className="text-sm">
                              并发数
                            </Label>
                            <Input
                              id={`${platform}-concurrency`}
                              type="number"
                              min="1"
                              max="100"
                              value={platformConfig.concurrency}
                              onChange={(event) =>
                                updateRateConfig(platform, 'concurrency', parseInt(event.target.value, 10) || 1)
                              }
                              className="h-9"
                              disabled={!platformConfig.enabled}
                            />
                            <p className="text-xs text-muted-foreground">同时检测的链接数量。</p>
                          </div>

                          <div className="space-y-1.5">
                            <Label htmlFor={`${platform}-delay`} className="text-sm">
                              请求间隔（毫秒）
                            </Label>
                            <Input
                              id={`${platform}-delay`}
                              type="number"
                              min="0"
                              max="10000"
                              value={platformConfig.request_delay_ms}
                              onChange={(event) =>
                                updateRateConfig(platform, 'request_delay_ms', parseInt(event.target.value, 10) || 0)
                              }
                              className="h-9"
                              disabled={!platformConfig.enabled}
                            />
                          </div>

                          <div className="space-y-1.5">
                            <Label htmlFor={`${platform}-max-rps`} className="text-sm">
                              每秒最大请求数
                            </Label>
                            <Input
                              id={`${platform}-max-rps`}
                              type="number"
                              min="0"
                              max="100"
                              value={platformConfig.max_requests_per_second}
                              onChange={(event) =>
                                updateRateConfig(platform, 'max_requests_per_second', parseInt(event.target.value, 10) || 0)
                              }
                              className="h-9"
                              disabled={!platformConfig.enabled}
                            />
                            <p className="text-xs text-muted-foreground">0 表示不限制。</p>
                          </div>
                        </div>
                      </div>
                    </Card>
                  );
                })}
              </div>
              <div className="flex justify-end">
                <Button onClick={handleSaveRateConfig} disabled={savingRate} className="w-32">
                  {savingRate ? '保存中...' : '保存所有'}
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
