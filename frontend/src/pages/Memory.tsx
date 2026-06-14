import { useEffect, useState } from 'react';
import { api } from '@/api/authApi';
import { toast } from 'sonner';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { PLATFORM_NAMES } from '@/utils/constants';

interface MemoryOverview {
  history_ttl_minutes: number;
  cleanup_interval_minutes: number;
  invalid_links_total: number;
  checked_links_total: number;
  next_cleanup_at: string;
}

interface InvalidLinkItem {
  link: string;
  platform: string;
  failure_reason: string;
  is_rate_limited: boolean;
  created_at: string;
  query_time: string;
  cleanup_at: string;
}

interface CheckedLinkItem {
  link: string;
  valid: boolean;
  query_time: string;
  cleanup_at: string;
}

interface MemoryData {
  overview: MemoryOverview;
  invalid_links: InvalidLinkItem[];
  checked_links: CheckedLinkItem[];
}

function formatTime(t: string): string {
  if (!t) return '-';
  return new Date(t).toLocaleString('zh-CN');
}

export function Memory() {
  const [data, setData] = useState<MemoryData | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const res = await api.get<{ data: MemoryData }>('/memory/overview');
      setData(res.data.data);
    } catch (err: any) {
      toast.error('加载内存数据失败: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">内存数据</h1>
          <p className="text-sm text-muted-foreground mt-1">
            每 {data?.overview.cleanup_interval_minutes ?? '-'} 分钟清理一次，
            超过 {data?.overview.history_ttl_minutes ?? '-'} 分钟未使用的数据将被删除
          </p>
        </div>
        <button
          className="px-3 py-1.5 text-sm border rounded-md hover:bg-accent"
          onClick={load}
          disabled={loading}
        >
          {loading ? '加载中...' : '刷新'}
        </button>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">失效链接</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data?.overview.invalid_links_total ?? '-'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">检测缓存</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data?.overview.checked_links_total ?? '-'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">保留时间</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{Math.round((data?.overview.history_ttl_minutes ?? 0) / 60)}小时</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">下次清理</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-lg font-bold">
              {data?.overview.next_cleanup_at ? formatTime(data.overview.next_cleanup_at) : '-'}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>失效链接 ({data?.overview.invalid_links_total ?? 0})</CardTitle>
          <CardDescription>已标记为失效或被限流的链接，含查询时间和预计清理时间</CardDescription>
        </CardHeader>
        <CardContent className="max-h-[500px] overflow-auto">
          {loading || !data?.invalid_links?.length ? (
            <div className="flex h-32 items-center justify-center text-muted-foreground">
              {loading ? '加载中...' : '暂无数据'}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[360px]">链接</TableHead>
                  <TableHead>平台</TableHead>
                  <TableHead>原因</TableHead>
                  <TableHead>是否限流</TableHead>
                  <TableHead>查询时间</TableHead>
                  <TableHead>清理时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.invalid_links.map((item, i) => (
                  <TableRow key={i}>
                    <TableCell className="max-w-[360px] truncate">{item.link}</TableCell>
                    <TableCell>{PLATFORM_NAMES[item.platform] || item.platform}</TableCell>
                    <TableCell className="max-w-[200px] truncate">{item.failure_reason || '-'}</TableCell>
                    <TableCell>{item.is_rate_limited ? '是' : '否'}</TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatTime(item.query_time)}</TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatTime(item.cleanup_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>检测缓存 ({data?.overview.checked_links_total ?? 0})</CardTitle>
          <CardDescription>已缓存的检测结果，含查询时间和预计清理时间</CardDescription>
        </CardHeader>
        <CardContent className="max-h-[500px] overflow-auto">
          {loading || !data?.checked_links?.length ? (
            <div className="flex h-32 items-center justify-center text-muted-foreground">
              {loading ? '加载中...' : '暂无数据'}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[400px]">链接</TableHead>
                  <TableHead>是否有效</TableHead>
                  <TableHead>查询时间</TableHead>
                  <TableHead>清理时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.checked_links.map((item, i) => (
                  <TableRow key={i}>
                    <TableCell className="max-w-[400px] truncate">{item.link}</TableCell>
                    <TableCell>{item.valid ? '是' : '否'}</TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatTime(item.query_time)}</TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatTime(item.cleanup_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
