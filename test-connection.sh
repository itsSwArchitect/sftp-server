#!/bin/bash

# SFTP Connection Troubleshooting Script
# This script helps diagnose common SFTP/SSH connection issues

echo "=== SFTP Connection Troubleshooting ==="
echo

# Get connection details
read -p "Enter your VM's IP address or hostname: " VM_HOST
read -p "Enter SSH port (default 22): " VM_PORT
VM_PORT=${VM_PORT:-22}
read -p "Enter username: " VM_USER

echo
echo "=== Testing Connection to $VM_HOST:$VM_PORT ==="
echo

# Test 1: Basic network connectivity
echo "1. Testing network connectivity..."
if ping -c 3 "$VM_HOST" >/dev/null 2>&1; then
    echo "✅ Host $VM_HOST is reachable"
else
    echo "❌ Host $VM_HOST is NOT reachable"
    echo "   - Check if the VM is running"
    echo "   - Verify the IP address is correct"
    echo "   - Check network connectivity"
    exit 1
fi

# Test 2: Port connectivity
echo
echo "2. Testing SSH port connectivity..."
if nc -z -w5 "$VM_HOST" "$VM_PORT" 2>/dev/null; then
    echo "✅ Port $VM_PORT is open on $VM_HOST"
else
    echo "❌ Port $VM_PORT is NOT accessible on $VM_HOST"
    echo "   - Check if SSH service is running: sudo systemctl status ssh"
    echo "   - Check if SSH is listening: sudo netstat -tlnp | grep :$VM_PORT"
    echo "   - Check firewall settings: sudo ufw status"
    exit 1
fi

# Test 3: SSH service detection
echo
echo "3. Testing SSH service..."
ssh_banner=$(timeout 5 telnet "$VM_HOST" "$VM_PORT" 2>/dev/null | head -1)
if [[ $ssh_banner == *"SSH"* ]]; then
    echo "✅ SSH service detected: $ssh_banner"
else
    echo "⚠️  Could not detect SSH banner (this might be normal)"
fi

# Test 4: SSH authentication test
echo
echo "4. Testing SSH authentication..."
echo "   Attempting SSH connection (you'll be prompted for password)..."
if ssh -o ConnectTimeout=10 -o BatchMode=yes "$VM_USER@$VM_HOST" -p "$VM_PORT" "echo 'SSH connection successful'" 2>/dev/null; then
    echo "✅ SSH key authentication works"
elif ssh -o ConnectTimeout=10 "$VM_USER@$VM_HOST" -p "$VM_PORT" "echo 'SSH connection successful'"; then
    echo "✅ SSH password authentication works"
else
    echo "❌ SSH authentication failed"
    echo "   - Verify username: $VM_USER"
    echo "   - Check password"
    echo "   - Verify SSH is configured to allow password authentication"
    echo "   - Check /etc/ssh/sshd_config for PasswordAuthentication yes"
fi

echo
echo "=== Connection Summary ==="
echo "Host: $VM_HOST"
echo "Port: $VM_PORT"
echo "User: $VM_USER"
echo
echo "If all tests pass, try connecting through the web interface at:"
echo "http://localhost:8082"
echo
echo "Common VM SSH setup commands:"
echo "  sudo systemctl enable ssh"
echo "  sudo systemctl start ssh"
echo "  sudo ufw allow ssh"
echo "  sudo ufw allow $VM_PORT"
