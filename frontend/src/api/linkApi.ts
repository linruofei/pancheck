import axios from 'axios';
import type { CheckLinksRequest, CheckLinksResponse } from '@/types';

export const linkApi = {
  checkLinks: async (data: CheckLinksRequest): Promise<CheckLinksResponse> => {
    const response = await axios.post<CheckLinksResponse>('/api/v1/links/check', data, {
      timeout: 300000,
    });
    return response.data;
  },
};
