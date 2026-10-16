{ pkgs ? import <nixpkgs> {} }:

(pkgs.buildFHSEnv {
  name = "vivado-env";

  targetPkgs = pkgs: (with pkgs; [
    ncurses5
    zlib libuuid
    bash
    coreutils
    util-linux
    stdenv.cc.cc

    xorg.libXext
    xorg.libX11
    xorg.libXrender
    xorg.libXtst
    xorg.libXi
    xorg.libXft
    xorg.libxcb

    freetype
    fontconfig
    glib
    gtk2
    gtk3

    graphviz
    gcc
    unzip
    nettools
  ]);

  runScript = ''
    bash -c '
      export PATH=/run/media/jhush/OLDTING/Xilinx/Vivado/2019.2/bin:$PATH
      export LIBRARY_PATH=/usr/lib
      export C_INCLUDE_PATH=/usr/include
      export CPLUS_INCLUDE_PATH=/usr/include
      export CMAKE_LIBRARY_PATH=/usr/lib
      export CMAKE_INCLUDE_PATH=/usr/include
      export VCONNECT_BOOTSTRAP=/home/jhush/vconnect/tcl/bootstrap.tcl
      exec /home/jhush/vconnect/vconnect \
        -vivado /run/media/jhush/OLDTING/Xilinx/Vivado/2019.2/bin/vivado \
        -runs-dir /home/jhush/vconnect/runs \
        -log /home/jhush/vconnect/vconnect.log
    '
  '';
}).env
