const sharp = require('sharp');
const pngToIco = require('png-to-ico').default;
const fs = require('fs');
const path = require('path');

const svgPath = path.join(__dirname, 'icon.svg');
const svgBuffer = fs.readFileSync(svgPath);

async function generateIcons() {
  // 1. 生成 build/appicon.png (1024x1024，Wails 主图标)
  const appIconPng = await sharp(svgBuffer)
    .resize(1024, 1024)
    .png()
    .toBuffer();
  fs.writeFileSync(path.join(__dirname, '..', 'build', 'appicon.png'), appIconPng);
  console.log('✅ build/appicon.png (1024x1024) generated');

  // 2. 生成通用 logo
  const logoPng = await sharp(svgBuffer)
    .resize(512, 512)
    .png()
    .toBuffer();
  fs.writeFileSync(path.join(__dirname, '..', 'frontend', 'src', 'assets', 'images', 'logo-universal.png'), logoPng);
  console.log('✅ frontend/src/assets/images/logo-universal.png (512x512) generated');

  // 3. 生成多尺寸 PNG 用于 ICO 转换
  const sizes = [256, 128, 64, 48, 32, 16];
  const pngBuffers = {};
  for (const size of sizes) {
    pngBuffers[size] = await sharp(svgBuffer)
      .resize(size, size)
      .png()
      .toBuffer();
  }

  // 4. 生成 build/windows/icon.ico
  const windowsIco = await pngToIco(
    sizes.map(s => {
      const tmp = path.join(__dirname, `tmp-${s}.png`);
      fs.writeFileSync(tmp, pngBuffers[s]);
      return tmp;
    })
  );
  fs.writeFileSync(path.join(__dirname, '..', 'build', 'windows', 'icon.ico'), windowsIco);
  console.log('✅ build/windows/icon.ico generated');

  // 5. 生成 frontend/public/favicon.ico (通常不需要 256，小一点)
  const faviconSizes = [128, 64, 48, 32, 16];
  const favIco = await pngToIco(
    faviconSizes.map(s => path.join(__dirname, `tmp-${s}.png`))
  );
  fs.writeFileSync(path.join(__dirname, '..', 'frontend', 'public', 'favicon.ico'), favIco);
  console.log('✅ frontend/public/favicon.ico generated');

  // 清理临时文件
  sizes.forEach(s => {
    const tmp = path.join(__dirname, `tmp-${s}.png`);
    if (fs.existsSync(tmp)) fs.unlinkSync(tmp);
  });

  console.log('\n🎉 All icons generated successfully!');
}

generateIcons().catch(err => {
  console.error('❌ Failed to generate icons:', err);
  process.exit(1);
});
