export type Platform =
  | 'quark'
  | 'uc'
  | 'baidu'
  | 'tianyi'
  | 'pan123'
  | 'pan115'
  | 'aliyun'
  | 'xunlei'
  | 'cmcc'
  | 'unknown';

export interface CheckLinksRequest {
  links: string[];
  selected_platforms?: Platform[];
}

export interface CheckLinksResponse {
  invalid_links: string[];
  pending_links: string[];
  valid_links: string[];
  total_duration?: number;
  invalid_format_count: number;
  duplicate_count: number;
}

export interface LinkInfo {
  link: string;
  platform: Platform;
  status?: 'valid' | 'invalid' | 'pending';
}
