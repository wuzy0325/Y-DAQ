import { readFileSync, writeFileSync, readdirSync, statSync } from 'fs'
import { join, extname } from 'path'
import { fileURLToPath } from 'url'

const bindingsDir = fileURLToPath(new URL('../bindings', import.meta.url))

function fixExtension(filePath) {
  const content = readFileSync(filePath, 'utf-8')
  // 匹配相对路径导入（./ 和 ../）中多余的 .js 扩展名
  const updated = content.replace(/(from\s+["'])(\.\.?\/[^"']+)\.js(["'])/g, '$1$2$3')
  if (updated !== content) {
    writeFileSync(filePath, updated, 'utf-8')
    console.log(`  fixed: ${filePath}`)
  }
}

function walk(dir) {
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry)
    if (statSync(fullPath).isDirectory()) {
      walk(fullPath)
    } else if (extname(fullPath) === '.ts' || extname(fullPath) === '.js') {
      fixExtension(fullPath)
    }
  }
}

walk(bindingsDir)
console.log('Bindings imports fixed.')
