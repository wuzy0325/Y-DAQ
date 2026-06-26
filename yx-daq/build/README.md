# Build Directory

The build directory is used to house build configuration files and assets for the Wails v3 application.

The structure is:

* `../bin` - Output directory for `yx-daq.exe` and `yx-daq-amd64-installer.exe`
* darwin - macOS specific files
* windows - Windows specific files

## Mac

The `darwin` directory holds files specific to Mac builds.
These may be customised and used as part of the build. To return these files to the default state, simply delete them
and
build with `wails3 task build`.

The directory contains the following files:

- `Info.plist` - the main plist file used for Mac builds.
- `Info.dev.plist` - same as the main plist file but used for development builds.

## Windows

The `windows` directory contains the manifest, icon, resource and installer files used by the Wails v3 Taskfile.
These may be customised for your application. To return these files to the default state, simply delete them and
build with `wails3 task build`.

- `icon.ico` - The icon used for the application. If you wish to
  use a different icon, simply replace this file with your own. If it is missing, a new `icon.ico` file
  will be created using the `appicon.png` file in the build directory.
- `nsis/*` - The files used to create the Windows NSIS installer via `build.bat nsis` or `wails3 task package`.
- `info.json` - Application details used for Windows builds. The data here will be used by the Windows installer,
  as well as the application itself (right click the exe -> properties -> details)
- `wails.exe.manifest` - The main application manifest file.
