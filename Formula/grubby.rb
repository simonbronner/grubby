class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "9"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v9/grubby_v9_darwin_arm64.tar.gz"
      sha256 "dd95b3066f75e8ddafdca48766454a4e591751b8ba1efd876a7f3144017c214d"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v9/grubby_v9_darwin_amd64.tar.gz"
      sha256 "eb3f72762bcbbd6dae028219ee0a24dc32deef3e76014624bdc4586214c4e563"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v9/grubby_v9_linux_arm64.tar.gz"
      sha256 "73c17d466282a6a073ad7ad01ff1b181be2988d3280d7492ee7ac3003f85342e"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v9/grubby_v9_linux_amd64.tar.gz"
      sha256 "467be6fb0301d1255d8e934c858c128a645c7f678b1317d6df660c34e1c1907c"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
