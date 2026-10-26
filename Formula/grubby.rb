class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "16"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v16/grubby_v16_darwin_arm64.tar.gz"
      sha256 "2d114b6f50b92a16ad3d5a7575a5aafe16bec43dbafcc2e88d349de7ff1b7c5c"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v16/grubby_v16_darwin_amd64.tar.gz"
      sha256 "be8c98c16b9d2c26ffa65754c7238dcbc7d0cbf7536e732971791a869aef01f8"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v16/grubby_v16_linux_arm64.tar.gz"
      sha256 "ddadd0a716e6581a24563e09c4a890999ecb8fa6d50253b2f93089bab62cdb9a"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v16/grubby_v16_linux_amd64.tar.gz"
      sha256 "d0234f5b3b813d7bee761a2f17d82acc1bf47fb16ce3e37c026434184d275206"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
