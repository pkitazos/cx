# From your project root, create a test directory
mkdir -p test/source/nested
mkdir -p test/destination

# Create some test files
echo "This is file 1" > test/source/file1.txt
echo "This is file 2" > test/source/file2.txt
echo "This is a nested file" > test/source/nested/file3.txt

# Create a test directory with multiple files
mkdir -p test/source/config
echo '{"setting": "value"}' > test/source/config/settings.json
echo 'key=value' > test/source/config/config.ini