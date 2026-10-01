#!/usr/bin/env bash
# cloudsprt_bootstrap.sh — ONE-TIME account bootstrap for scripts/cloudsprt.sh.
#
# Run this in AWS CloudShell (the >_ icon in the console toolbar) while logged
# in to the console — NOT on the Mac, and never by minting root access keys.
# Re-runnable; every step is idempotent.
#
# What it creates (least privilege, escalation closed):
#   cloudsprt-role     worker role: s3:PutObject to cloudsprt-* buckets, NOTHING else
#   cloudsprt-profile  instance profile wrapping that role
#   cloudsprt (user)   the operator key the Mac CLI uses. It can ONLY:
#                        - launch SPOT instances of an explicit small-Graviton
#                          type allowlist (t4g.nano ... c8g.4xlarge) — on-demand
#                          and big/x86 types are DENIED by IAM condition
#                        - terminate instances TAGGED app=cloudsprt (no others)
#                        - read/write cloudsprt-* S3 buckets only
#                        - pass only cloudsprt-role (whose only power is PutObject)
#                        - NO iam:Create*/Put*/Attach* => cannot grant itself more
#   budget             $20/mo account cost budget, email at 50%/100% actual
#                      + 100% forecast
#
# It prints the operator access key LAST — paste those two values into
# `aws configure` on the Mac. The secret is shown exactly once.

set -uo pipefail

EMAIL=${EMAIL:-ehrlich.bryan@gmail.com}
ACCT=$(aws sts get-caller-identity --query Account --output text) || { echo "no credentials?"; exit 1; }
echo "account $ACCT, alerts to $EMAIL"

# --- worker role (what instances run as: PutObject-only) ---------------------
aws iam create-role --role-name cloudsprt-role --assume-role-policy-document \
  '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}' \
  >/dev/null 2>&1 && echo "worker role: created" || echo "worker role: exists"
aws iam put-role-policy --role-name cloudsprt-role --policy-name s3put --policy-document \
  '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:PutObject","Resource":"arn:aws:s3:::cloudsprt-*/*"}]}'
aws iam create-instance-profile --instance-profile-name cloudsprt-profile \
  >/dev/null 2>&1 && echo "instance profile: created" || echo "instance profile: exists"
aws iam add-role-to-instance-profile --instance-profile-name cloudsprt-profile --role-name cloudsprt-role \
  >/dev/null 2>&1 || true

# --- operator user (what the Mac CLI runs as) --------------------------------
aws iam create-user --user-name cloudsprt \
  >/dev/null 2>&1 && echo "operator user: created" || echo "operator user: exists"

aws iam put-user-policy --user-name cloudsprt --policy-name cloudsprt-operator --policy-document '{
  "Version": "2012-10-17",
  "Statement": [
    { "Sid": "Ec2Read",
      "Effect": "Allow",
      "Action": "ec2:Describe*",
      "Resource": "*" },
    { "Sid": "OwnKeypairOnly",
      "Effect": "Allow",
      "Action": ["ec2:CreateKeyPair", "ec2:DeleteKeyPair"],
      "Resource": "arn:aws:ec2:*:*:key-pair/cloudsprt" },
    { "Sid": "SecurityGroup",
      "Effect": "Allow",
      "Action": ["ec2:CreateSecurityGroup", "ec2:AuthorizeSecurityGroupIngress"],
      "Resource": "*" },
    { "Sid": "RunSpotSmallGravitonOnly",
      "Effect": "Allow",
      "Action": "ec2:RunInstances",
      "Resource": "arn:aws:ec2:*:*:instance/*",
      "Condition": {
        "StringEquals": {
          "ec2:InstanceMarketType": "spot",
          "ec2:InstanceType": [
            "t4g.nano", "t4g.micro",
            "c7g.xlarge", "c7g.2xlarge", "c7g.4xlarge",
            "c8g.xlarge", "c8g.2xlarge", "c8g.4xlarge"
          ]
        }
      } },
    { "Sid": "RunSupportingResources",
      "Effect": "Allow",
      "Action": "ec2:RunInstances",
      "Resource": [
        "arn:aws:ec2:*:*:image/*",
        "arn:aws:ec2:*:*:key-pair/cloudsprt",
        "arn:aws:ec2:*:*:security-group/*",
        "arn:aws:ec2:*:*:subnet/*",
        "arn:aws:ec2:*:*:network-interface/*",
        "arn:aws:ec2:*:*:volume/*",
        "arn:aws:ec2:*:*:spot-instances-request/*"
      ] },
    { "Sid": "TagOnLaunchOnly",
      "Effect": "Allow",
      "Action": "ec2:CreateTags",
      "Resource": "*",
      "Condition": { "StringEquals": { "ec2:CreateAction": "RunInstances" } } },
    { "Sid": "TerminateOnlyOurs",
      "Effect": "Allow",
      "Action": "ec2:TerminateInstances",
      "Resource": "*",
      "Condition": { "StringEquals": { "aws:ResourceTag/app": "cloudsprt" } } },
    { "Sid": "PassOnlyWorkerRole",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::*:role/cloudsprt-role" },
    { "Sid": "IamReadChecks",
      "Effect": "Allow",
      "Action": ["iam:GetRole", "iam:GetInstanceProfile"],
      "Resource": "*" },
    { "Sid": "AmiLookup",
      "Effect": "Allow",
      "Action": "ssm:GetParameter",
      "Resource": "arn:aws:ssm:*::parameter/aws/service/ami-amazon-linux-latest/*" },
    { "Sid": "OwnBuckets",
      "Effect": "Allow",
      "Action": ["s3:CreateBucket", "s3:ListBucket", "s3:GetBucketLocation", "s3:PutLifecycleConfiguration"],
      "Resource": "arn:aws:s3:::cloudsprt-*" },
    { "Sid": "OwnObjects",
      "Effect": "Allow",
      "Action": ["s3:PutObject", "s3:GetObject"],
      "Resource": "arn:aws:s3:::cloudsprt-*/*" }
  ]
}'
echo "operator policy: applied (spot-only, small-Graviton allowlist, tag-scoped terminate, no iam writes)"

# --- $20/mo budget with email alerts -----------------------------------------
aws budgets create-budget --account-id "$ACCT" \
  --budget '{"BudgetName":"cloudsprt-guard","BudgetLimit":{"Amount":"20","Unit":"USD"},"TimeUnit":"MONTHLY","BudgetType":"COST"}' \
  --notifications-with-subscribers "[
    {\"Notification\":{\"NotificationType\":\"ACTUAL\",\"ComparisonOperator\":\"GREATER_THAN\",\"Threshold\":50},\"Subscribers\":[{\"SubscriptionType\":\"EMAIL\",\"Address\":\"$EMAIL\"}]},
    {\"Notification\":{\"NotificationType\":\"ACTUAL\",\"ComparisonOperator\":\"GREATER_THAN\",\"Threshold\":100},\"Subscribers\":[{\"SubscriptionType\":\"EMAIL\",\"Address\":\"$EMAIL\"}]},
    {\"Notification\":{\"NotificationType\":\"FORECASTED\",\"ComparisonOperator\":\"GREATER_THAN\",\"Threshold\":100},\"Subscribers\":[{\"SubscriptionType\":\"EMAIL\",\"Address\":\"$EMAIL\"}]}]" \
  >/dev/null 2>&1 && echo "budget: created (\$20/mo, alerts at \$10/\$20/forecast)" || echo "budget: exists"

# --- informational: the structural concurrency ceiling ------------------------
Q=$(aws service-quotas get-service-quota --service-code ec2 --quota-code L-34B43A08 \
    --query 'Quota.Value' --output text 2>/dev/null || echo "?")
echo "spot vCPU quota in ${AWS_REGION:-this region}: $Q (hard cap on concurrent burn)"

# --- the operator access key (shown ONCE) -------------------------------------
NKEYS=$(aws iam list-access-keys --user-name cloudsprt --query 'length(AccessKeyMetadata)' --output text)
if [ "$NKEYS" = "0" ]; then
  echo
  echo "=== paste these two into 'aws configure' on the Mac (AccessKeyId then Secret) ==="
  aws iam create-access-key --user-name cloudsprt \
    --query 'AccessKey.[AccessKeyId,SecretAccessKey]' --output text
else
  echo "operator already has an access key — to rotate: aws iam delete-access-key --user-name cloudsprt --access-key-id <id>, then re-run"
fi
