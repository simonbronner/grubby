class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "5"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v5/grubby_v5_darwin_arm64.tar.gz"
      sha256 "602e55b17335b26cb1170a8d829861c7c3cc54b9e293cec7fa347285c95d7154"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v5/grubby_v5_darwin_amd64.tar.gz"
      sha256 "bdab7c7820ecf25fcba68e8dc68291605495a671d0bf830a7c4d551ba336d007"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v5/grubby_v5_linux_arm64.tar.gz"
      sha256 "a9571b5660318c7f16f269c8143f18606d4fb2e5c98701fa7aaceb2b25664038"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v5/grubby_v5_linux_amd64.tar.gz"
      sha256 "cfc628d9f167fb1493a0d11184c77864437bef1927f091428db0cb25ba3b4394"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
