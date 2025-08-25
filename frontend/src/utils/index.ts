export function formatTimestamp(timestamp: string | null): string {
  if (!timestamp) return '-';
  return new Date(timestamp).toLocaleString('zh-CN');
}

export function getTimeDiff(timestamp: string | null): string {
  if (!timestamp) return '未知';
  const diff = Date.now() - new Date(timestamp).getTime();
  const minutes = Math.floor(diff / (1000 * 60));
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  
  if (days > 0) return `${days}天前`;
  if (hours > 0) return `${hours}小时前`;
  if (minutes > 0) return `${minutes}分钟前`;
  return '刚刚';
}

export function getTimeDiffClass(timestamp: string | null): string {
  if (!timestamp) return 'bg-gray-100 text-gray-600';
  const diff = Date.now() - new Date(timestamp).getTime();
  const minutes = diff / (1000 * 60);
  
  if (minutes < 5) return 'bg-green-100 text-green-800';
  if (minutes < 30) return 'bg-yellow-100 text-yellow-800';
  return 'bg-red-100 text-red-800';
} 