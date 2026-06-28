export function downloadCSV(filenamePrefix: string, headers: string[], rows: (string | number)[][]): void {
  const BOM = '\uFEFF'
  const csv = BOM + headers.join(',') + '\n' + rows.map(r => r.join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${filenamePrefix}-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
