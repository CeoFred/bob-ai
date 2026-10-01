export function cn(...classes: (string | undefined | null | false | Record<string, boolean>)[]) {
  const result: string[] = [];
  for (const item of classes) {
    if (!item) continue;
    if (typeof item === 'string') {
      result.push(item);
    } else if (typeof item === 'object') {
      for (const key in item) {
        if (item[key]) result.push(key);
      }
    }
  }
  return result.join(' ');
}
