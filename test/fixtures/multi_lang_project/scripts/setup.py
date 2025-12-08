#!/usr/bin/env python3
"""
Setup script for the Python automation tools.

This module provides installation and configuration for the Python
components of the multi-language project.
"""

from setuptools import setup, find_packages

setup(
    name="multi-lang-helpers",
    version="1.0.0",
    description="Python automation helpers for multi-language project",
    author="Test Author",
    author_email="test@example.com",
    python_requires=">=3.9",
    packages=find_packages(),
    install_requires=[
        "requests>=2.31.0",
        "pyyaml>=6.0",
        "click>=8.1.0",
    ],
    entry_points={
        "console_scripts": [
            "mlh-migrate=helpers:migrate",
            "mlh-validate=helpers:validate",
        ],
    },
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
)
